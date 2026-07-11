package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/quotaview"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetUserPlatformQuotas GET /admin/users/:id/platform-quotas
// admin 视角：D14 lazy 归零 + 暴露 *_window_start 调试字段
func (h *UserHandler) GetUserPlatformQuotas(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}
	if h.userPlatformQuotaRepo == nil {
		response.Success(c, map[string]any{"platform_quotas": []any{}})
		return
	}
	// 校验用户存在：与 PUT/POST 路径一致，不存在返回 404 而非空数组（避免 admin 界面误判用户存在）。
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	records, err := h.userPlatformQuotaRepo.ListByUser(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	now := time.Now().UTC()
	out := make([]map[string]any, 0, len(records))
	for _, r := range records {
		out = append(out, quotaview.LazyZeroQuotaForResponse(r, now, true)) // true = 暴露 window_start
	}
	response.Success(c, map[string]any{"platform_quotas": out})
}

// UpdateUserPlatformQuotasRequest is the body for PUT /admin/users/:id/platform-quotas.
type UpdateUserPlatformQuotasRequest struct {
	Quotas []PlatformQuotaInput `json:"quotas" binding:"required"`
}

// PlatformQuotaInput 单平台限额输入；limit 字段为 nil 表示不限制。
type PlatformQuotaInput struct {
	Platform        string   `json:"platform" binding:"required"`
	DailyLimitUSD   *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
}

// platform 合法性由 service.IsAllowedQuotaPlatform / service.AllowedQuotaPlatforms 统一判断（单一源）。

// UpdateUserPlatformQuotas PUT /admin/users/:id/platform-quotas
// 全量替换该用户所有平台限额。
func (h *UserHandler) UpdateUserPlatformQuotas(c *gin.Context) {
	if h.userPlatformQuotaRepo == nil {
		response.Error(c, 503, "platform quota service not available")
		return
	}

	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	var req UpdateUserPlatformQuotasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.Quotas) > len(service.AllowedQuotaPlatforms) {
		response.BadRequest(c, fmt.Sprintf("quotas length must be <= %d", len(service.AllowedQuotaPlatforms)))
		return
	}
	seen := make(map[string]struct{}, len(req.Quotas))
	for _, q := range req.Quotas {
		if !service.IsAllowedQuotaPlatform(q.Platform) {
			response.BadRequest(c, "invalid platform: "+q.Platform)
			return
		}
		if _, dup := seen[q.Platform]; dup {
			response.BadRequest(c, "duplicate platform: "+q.Platform)
			return
		}
		seen[q.Platform] = struct{}{}
		// daily_limit_usd / weekly_limit_usd / monthly_limit_usd 的语义：
		//   nil / not set → 无限额（完全放行）
		//   0            → 完全禁用（任何请求都会被拒绝，因为 usage >= 0 恒成立）
		//   > 0          → USD 限额上限
		// 拦截 NaN / ±Inf：客户端可发送超大数（如 1e308 × 2）使 JSON 反序列化得到 +Inf，
		// 进入 DB 后 cache check 中 usage >= limit 永不成立，limit 等同失效。
		for _, f := range []struct {
			name string
			val  *float64
		}{
			{"daily_limit_usd", q.DailyLimitUSD},
			{"weekly_limit_usd", q.WeeklyLimitUSD},
			{"monthly_limit_usd", q.MonthlyLimitUSD},
		} {
			if f.val == nil {
				continue
			}
			v := *f.val
			if v < 0 {
				response.BadRequest(c, f.name+" must be >= 0")
				return
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				response.BadRequest(c, f.name+" must be a finite number")
				return
			}
		}
	}

	records := make([]service.UserPlatformQuotaRecord, 0, len(req.Quotas))
	for _, q := range req.Quotas {
		records = append(records, service.UserPlatformQuotaRecord{
			UserID:          userID,
			Platform:        q.Platform,
			DailyLimitUSD:   q.DailyLimitUSD,
			WeeklyLimitUSD:  q.WeeklyLimitUSD,
			MonthlyLimitUSD: q.MonthlyLimitUSD,
		})
	}

	ctx := c.Request.Context()
	// 校验用户是否存在，避免 FK 违反导致 500；用户不存在时返回 404。
	if _, err := h.adminService.GetUser(ctx, userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 在 UpsertForUser 之前抓取 before snapshot 用于审计 before/after 对比。
	// ListByUser 失败不阻断主操作（best-effort），仅记录降级 warn。
	beforeRecords, beforeErr := h.userPlatformQuotaRepo.ListByUser(ctx, userID)
	if beforeErr != nil {
		slog.Warn("quota audit before snapshot failed", "user_id", userID, "err", beforeErr)
	}
	if err := h.userPlatformQuotaRepo.UpsertForUser(ctx, userID, records); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	beforeByPlatform := make(map[string]service.UserPlatformQuotaRecord, len(beforeRecords))
	for _, r := range beforeRecords {
		beforeByPlatform[r.Platform] = r
	}
	afterPlatforms := make(map[string]struct{}, len(records))
	for _, r := range records {
		afterPlatforms[r.Platform] = struct{}{}
	}
	changes := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"platform":          r.Platform,
			"daily_limit_usd":   r.DailyLimitUSD,
			"weekly_limit_usd":  r.WeeklyLimitUSD,
			"monthly_limit_usd": r.MonthlyLimitUSD,
		}
		if prev, ok := beforeByPlatform[r.Platform]; ok {
			entry["before_daily_limit_usd"] = prev.DailyLimitUSD
			entry["before_weekly_limit_usd"] = prev.WeeklyLimitUSD
			entry["before_monthly_limit_usd"] = prev.MonthlyLimitUSD
		}
		changes = append(changes, entry)
	}
	// 补 removed 条目：before 存在但 after 缺失 = 该平台被软删除。
	// 缺少这条记录，审计消费方无法察觉"管理员把某平台从配额列表移除"的操作（合规盲区）。
	for _, prev := range beforeRecords {
		if _, kept := afterPlatforms[prev.Platform]; kept {
			continue
		}
		changes = append(changes, map[string]any{
			"platform":                 prev.Platform,
			"removed":                  true,
			"before_daily_limit_usd":   prev.DailyLimitUSD,
			"before_weekly_limit_usd":  prev.WeeklyLimitUSD,
			"before_monthly_limit_usd": prev.MonthlyLimitUSD,
		})
	}
	// before_snapshot_available 让审计消费方能识别 changes 中是否带 before_* 字段；
	// false 时所有 entry 都会缺失 before_*_limit_usd，仅有 after 视图。
	slog.Info("admin.quota_updated",
		"actor_admin_id", getAdminIDFromContext(c),
		"target_user_id", userID,
		"platform_count", len(records),
		"before_snapshot_available", beforeErr == nil,
		"changes", changes)

	// 失效 cache：对全部允许的 platform 统一 invalidate。
	// Trade-off：精确失效（仅 req 涉及平台 + 被软删平台）需 upsert 前额外 ListByUser，
	// 增加一次 DB 查询和逻辑复杂度。由于 AllowedQuotaPlatforms 数量很少，
	// 全量 invalidate 的额外开销可接受，且能可靠覆盖软删除场景。
	if h.billingCache != nil {
		for _, p := range service.AllowedQuotaPlatforms {
			if err := h.billingCache.DeleteUserPlatformQuotaCache(ctx, userID, p); err != nil {
				slog.Error("ALERT: quota cache invalidation failed after UpsertForUser; limit 生效可能延迟至 sentinel TTL(最长 1h),需人工确认或重试失效", "user_id", userID, "platform", p, "err", err)
			}
		}
	}

	// 返回最新状态
	now := time.Now().UTC()
	records2, err := h.userPlatformQuotaRepo.ListByUser(ctx, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]map[string]any, 0, len(records2))
	for i := range records2 {
		out = append(out, quotaview.LazyZeroQuotaForResponse(records2[i], now, true))
	}
	response.Success(c, map[string]any{"platform_quotas": out})
}

// ResetUserPlatformQuotaWindowRequest is the body for POST /admin/users/:id/platform-quotas/reset.
type ResetUserPlatformQuotaWindowRequest struct {
	Platform string `json:"platform" binding:"required"`
	Window   string `json:"window" binding:"required"`
}

var allowedWindowsForQuotaReset = map[string]struct{}{
	"daily":   {},
	"weekly":  {},
	"monthly": {},
}

// ResetUserPlatformQuotaWindow POST /admin/users/:id/platform-quotas/reset
// 立即归零指定 (platform, window) 的用量并更新 window_start。
func (h *UserHandler) ResetUserPlatformQuotaWindow(c *gin.Context) {
	if h.userPlatformQuotaRepo == nil {
		response.Error(c, 503, "platform quota service not available")
		return
	}

	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	var req ResetUserPlatformQuotaWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if !service.IsAllowedQuotaPlatform(req.Platform) {
		response.BadRequest(c, "invalid platform: "+req.Platform)
		return
	}
	if _, ok := allowedWindowsForQuotaReset[req.Window]; !ok {
		response.BadRequest(c, "invalid window: "+req.Window)
		return
	}

	ctx := c.Request.Context()
	// 校验用户是否存在，避免对不存在的用户执行操作返回误导性的 500。
	if _, err := h.adminService.GetUser(ctx, userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	now := time.Now().UTC()
	if err := h.userPlatformQuotaRepo.ResetExpiredWindow(ctx, userID, req.Platform, req.Window, now); err != nil {
		if errors.Is(err, service.ErrUserPlatformQuotaNotFound) {
			response.NotFound(c, "user platform quota not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	slog.Info("admin.quota_window_reset",
		"actor_admin_id", getAdminIDFromContext(c),
		"target_user_id", userID,
		"platform", req.Platform,
		"window", req.Window)

	if h.billingCache != nil {
		if err := h.billingCache.DeleteUserPlatformQuotaCache(ctx, userID, req.Platform); err != nil {
			slog.Error("ALERT: quota cache invalidation failed after ResetExpiredWindow; 窗口重置可能延迟至 sentinel TTL(最长 1h)", "user_id", userID, "platform", req.Platform, "err", err)
		}
	}

	records, err := h.userPlatformQuotaRepo.ListByUser(ctx, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]map[string]any, 0, len(records))
	for i := range records {
		out = append(out, quotaview.LazyZeroQuotaForResponse(records[i], now, true))
	}
	response.Success(c, map[string]any{"platform_quotas": out})
}
