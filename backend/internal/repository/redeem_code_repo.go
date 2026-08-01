package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"

	entsql "entgo.io/ent/dialect/sql"
)

// Path-safe prefix for codes that predate batch_id assignment.
const redeemLegacyBatchPrefix = "legacy-"

type redeemCodeRepository struct {
	client *dbent.Client
}

func NewRedeemCodeRepository(client *dbent.Client) service.RedeemCodeRepository {
	return &redeemCodeRepository{client: client}
}

func (r *redeemCodeRepository) Create(ctx context.Context, code *service.RedeemCode) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.RedeemCode.Create().
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetBatchRedeemLimitPerUser(code.BatchRedeemLimitPerUser).
		SetNillableUsedBy(code.UsedBy).
		SetNillableUsedAt(code.UsedAt).
		SetNillableExpiresAt(code.ExpiresAt).
		SetNillableBatchID(code.BatchID).
		SetNillableGroupID(code.GroupID).
		Save(ctx)
	if err == nil {
		code.ID = created.ID
		code.CreatedAt = created.CreatedAt
	}
	return err
}

func (r *redeemCodeRepository) CreateBatch(ctx context.Context, codes []service.RedeemCode) error {
	if len(codes) == 0 {
		return nil
	}

	client := clientFromContext(ctx, r.client)
	builders := make([]*dbent.RedeemCodeCreate, 0, len(codes))
	for i := range codes {
		c := &codes[i]
		b := client.RedeemCode.Create().
			SetCode(c.Code).
			SetType(c.Type).
			SetValue(c.Value).
			SetStatus(c.Status).
			SetNotes(c.Notes).
			SetValidityDays(c.ValidityDays).
			SetBatchRedeemLimitPerUser(c.BatchRedeemLimitPerUser).
			SetNillableUsedBy(c.UsedBy).
			SetNillableUsedAt(c.UsedAt).
			SetNillableExpiresAt(c.ExpiresAt).
			SetNillableBatchID(c.BatchID).
			SetNillableGroupID(c.GroupID)
		builders = append(builders, b)
	}

	return client.RedeemCode.CreateBulk(builders...).Exec(ctx)
}

func (r *redeemCodeRepository) GetByID(ctx context.Context, id int64) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) GetByCode(ctx context.Context, code string) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.CodeEQ(code)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.RedeemCode.Delete().Where(redeemcode.IDEQ(id)).Exec(ctx)
	return err
}

func (r *redeemCodeRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

func (r *redeemCodeRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query()

	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}
	if status != "" {
		q = q.Where(redeemcode.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(
			redeemcode.Or(
				redeemcode.CodeContainsFold(search),
				redeemcode.HasUserWith(user.EmailContainsFold(search)),
			),
		)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codesQuery := q.
		WithUser().
		WithGroup().
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range redeemCodeListOrder(params) {
		codesQuery = codesQuery.Order(order)
	}

	codes, err := codesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outCodes := redeemCodeEntitiesToService(codes)

	return outCodes, paginationResultFromTotal(int64(total), params), nil
}

func redeemCodeListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	var field string
	switch sortBy {
	case "type":
		field = redeemcode.FieldType
	case "value":
		field = redeemcode.FieldValue
	case "status":
		field = redeemcode.FieldStatus
	case "used_at":
		field = redeemcode.FieldUsedAt
	case "created_at":
		field = redeemcode.FieldCreatedAt
	case "code":
		field = redeemcode.FieldCode
	default:
		field = redeemcode.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(redeemcode.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(redeemcode.FieldID)}
}

func (r *redeemCodeRepository) Update(ctx context.Context, code *service.RedeemCode) error {
	up := r.client.RedeemCode.UpdateOneID(code.ID).
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetBatchRedeemLimitPerUser(code.BatchRedeemLimitPerUser)

	if code.UsedBy != nil {
		up.SetUsedBy(*code.UsedBy)
	} else {
		up.ClearUsedBy()
	}
	if code.UsedAt != nil {
		up.SetUsedAt(*code.UsedAt)
	} else {
		up.ClearUsedAt()
	}
	if code.BatchID != nil {
		up.SetBatchID(*code.BatchID)
	} else {
		up.ClearBatchID()
	}
	if code.ExpiresAt != nil {
		up.SetExpiresAt(*code.ExpiresAt)
	} else {
		up.ClearExpiresAt()
	}
	if code.GroupID != nil {
		up.SetGroupID(*code.GroupID)
	} else {
		up.ClearGroupID()
	}

	updated, err := up.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRedeemCodeNotFound
		}
		return err
	}
	code.CreatedAt = updated.CreatedAt
	return nil
}

func (r *redeemCodeRepository) BatchUpdate(ctx context.Context, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return 0, nil
	}
	client := clientFromContext(ctx, r.client)
	existing, err := client.RedeemCode.Query().Where(redeemcode.IDIn(unique...)).All(ctx)
	if err != nil {
		return 0, err
	}
	if len(existing) != len(unique) {
		return 0, service.ErrRedeemCodeNotFound
	}
	if fields.TouchesUsedSensitiveFields() {
		for _, code := range existing {
			if code.Status == service.StatusUsed {
				return 0, service.ErrRedeemCodeUsed
			}
		}
	}
	up := client.RedeemCode.Update().Where(redeemcode.IDIn(unique...))
	if fields.Status != nil {
		up.SetStatus(*fields.Status)
	}
	if fields.Notes != nil {
		up.SetNotes(*fields.Notes)
	}
	if fields.ExpiresAt.Set {
		if fields.ExpiresAt.Value != nil {
			up.SetExpiresAt(*fields.ExpiresAt.Value)
		} else {
			up.ClearExpiresAt()
		}
	}
	if fields.GroupID.Set {
		if fields.GroupID.Value != nil {
			up.SetGroupID(*fields.GroupID.Value)
		} else {
			up.ClearGroupID()
		}
	}
	affected, err := up.Save(ctx)
	if err != nil {
		return 0, err
	}
	if affected != len(unique) {
		return 0, service.ErrRedeemCodeNotFound
	}
	return int64(affected), nil
}

func (r *redeemCodeRepository) Use(ctx context.Context, id, userID int64) error {
	now := time.Now()
	client := clientFromContext(ctx, r.client)
	affected, err := client.RedeemCode.Update().
		Where(redeemcode.IDEQ(id), redeemcode.StatusEQ(service.StatusUnused)).
		SetStatus(service.StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(now).
		Save(ctx)
	if err != nil {
		if isRedeemBatchLimitConstraint(err) {
			return service.ErrRedeemBatchLimitExceeded
		}
		return err
	}
	if affected == 0 {
		return service.ErrRedeemCodeUsed
	}
	return nil
}

func (r *redeemCodeRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.RedeemCode, error) {
	if limit <= 0 {
		limit = 10
	}

	codes, err := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID)).
		WithGroup().
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return redeemCodeEntitiesToService(codes), nil
}

// ListByUserPaginated returns paginated balance/concurrency history for a user.
// Supports optional type filter (e.g. "balance", "admin_balance", "concurrency", "admin_concurrency", "subscription").
func (r *redeemCodeRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID))

	// Optional type filter
	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codes, err := q.
		WithGroup().
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return redeemCodeEntitiesToService(codes), paginationResultFromTotal(int64(total), params), nil
}

// type is balance/admin_balance).
func (r *redeemCodeRepository) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	var result []struct {
		Sum float64 `json:"sum"`
	}
	err := r.client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(userID),
			redeemcode.ValueGT(0),
			redeemcode.TypeIn("balance", "admin_balance"),
		).
		Aggregate(dbent.As(dbent.Sum(redeemcode.FieldValue), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

func redeemCodeEntityToService(m *dbent.RedeemCode) *service.RedeemCode {
	if m == nil {
		return nil
	}
	out := &service.RedeemCode{
		ID:                      m.ID,
		Code:                    m.Code,
		Type:                    m.Type,
		Value:                   m.Value,
		Status:                  m.Status,
		UsedBy:                  m.UsedBy,
		UsedAt:                  m.UsedAt,
		Notes:                   derefString(m.Notes),
		CreatedAt:               m.CreatedAt,
		ExpiresAt:               m.ExpiresAt,
		BatchID:                 m.BatchID,
		BatchRedeemLimitPerUser: m.BatchRedeemLimitPerUser,
		GroupID:                 m.GroupID,
		ValidityDays:            m.ValidityDays,
	}
	if m.Edges.User != nil {
		out.User = userEntityToService(m.Edges.User)
	}
	if m.Edges.Group != nil {
		out.Group = groupEntityToService(m.Edges.Group)
	}
	return out
}

func isRedeemBatchLimitConstraint(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr != nil {
		return pqErr.Code == "23505" && pqErr.Constraint == "redeemcode_batch_id_used_by"
	}
	return false
}

// ListBatches returns paginated generation batches (or legacy single-code pseudo-batches).
func (r *redeemCodeRepository) ListBatches(
	ctx context.Context,
	params pagination.PaginationParams,
	codeType, status, search string,
) ([]service.RedeemCodeBatch, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)

	whereRC, havingRC, args := buildRedeemBatchFilters("rc", codeType, status, search)
	_, havingOuter, _ := buildRedeemBatchFilters("", codeType, status, search)

	countQuery := `
SELECT COUNT(*) FROM (
  SELECT
    CASE
      WHEN rc.batch_id IS NOT NULL AND rc.batch_id <> '' THEN rc.batch_id
      ELSE '` + redeemLegacyBatchPrefix + `' || rc.id::text
    END AS batch_key
  FROM redeem_codes rc
  WHERE ` + whereRC + `
  GROUP BY 1
  ` + havingRC + `
) batches`

	var total int64
	countRows, err := client.QueryContext(ctx, countQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("count redeem batches: %w", err)
	}
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			_ = countRows.Close()
			return nil, nil, fmt.Errorf("scan redeem batch count: %w", err)
		}
	}
	if err := countRows.Close(); err != nil {
		return nil, nil, err
	}

	offset := params.Offset()
	limit := params.Limit()
	if limit <= 0 {
		limit = 20
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	listQuery := `
SELECT
  batch_key,
  MAX(real_batch_id) AS batch_id,
  BOOL_OR(is_legacy) AS is_legacy,
  MAX(type) AS type,
  MAX(value)::float8 AS value,
  MAX(group_id) AS group_id,
  COALESCE(MAX(group_name), '') AS group_name,
  COALESCE(MAX(validity_days), 30) AS validity_days,
  BOOL_OR(batch_redeem_limit_per_user) AS batch_redeem_limit_per_user,
  MAX(expires_at) AS expires_at,
  MIN(created_at) AS created_at,
  COUNT(*)::int AS total_count,
  COUNT(*) FILTER (WHERE status = 'unused')::int AS unused_count,
  COUNT(*) FILTER (WHERE status = 'used')::int AS used_count,
  COUNT(*) FILTER (WHERE status = 'expired')::int AS expired_count
FROM (
  SELECT
    CASE
      WHEN rc.batch_id IS NOT NULL AND rc.batch_id <> '' THEN rc.batch_id
      ELSE '` + redeemLegacyBatchPrefix + `' || rc.id::text
    END AS batch_key,
    rc.batch_id AS real_batch_id,
    (rc.batch_id IS NULL OR rc.batch_id = '') AS is_legacy,
    rc.type,
    rc.value,
    rc.group_id,
    g.name AS group_name,
    rc.validity_days,
    rc.batch_redeem_limit_per_user,
    rc.expires_at,
    rc.created_at,
    rc.status
  FROM redeem_codes rc
  LEFT JOIN groups g ON g.id = rc.group_id
  WHERE ` + whereRC + `
) filtered
GROUP BY batch_key
` + havingOuter + `
ORDER BY MIN(created_at) DESC, batch_key DESC
LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)

	rows, err := client.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("list redeem batches: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.RedeemCodeBatch, 0)
	for rows.Next() {
		var (
			b         service.RedeemCodeBatch
			batchID   sql.NullString
			groupID   sql.NullInt64
			expiresAt sql.NullTime
			groupName string
		)
		if err := rows.Scan(
			&b.BatchKey,
			&batchID,
			&b.IsLegacy,
			&b.Type,
			&b.Value,
			&groupID,
			&groupName,
			&b.ValidityDays,
			&b.BatchRedeemLimitPerUser,
			&expiresAt,
			&b.CreatedAt,
			&b.TotalCount,
			&b.UnusedCount,
			&b.UsedCount,
			&b.ExpiredCount,
		); err != nil {
			return nil, nil, fmt.Errorf("scan redeem batch: %w", err)
		}
		if batchID.Valid && batchID.String != "" {
			id := batchID.String
			b.BatchID = &id
		}
		if groupID.Valid {
			gid := groupID.Int64
			b.GroupID = &gid
		}
		b.GroupName = groupName
		if expiresAt.Valid {
			t := expiresAt.Time
			b.ExpiresAt = &t
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return out, paginationResultFromTotal(total, params), nil
}

// buildRedeemBatchFilters builds WHERE (row-level) and HAVING (batch-level status) clauses.
func buildRedeemBatchFilters(alias, codeType, status, search string) (where string, having string, args []any) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	parts := []string{"TRUE"}
	args = make([]any, 0, 3)
	argN := 1

	if codeType != "" {
		parts = append(parts, fmt.Sprintf("%stype = $%d", prefix, argN))
		args = append(args, codeType)
		argN++
	}
	if search != "" {
		like := "%" + search + "%"
		parts = append(parts, fmt.Sprintf("(%scode ILIKE $%d OR %sbatch_id ILIKE $%d)", prefix, argN, prefix, argN))
		args = append(args, like)
		argN++
	}

	statusCol := "status"
	if alias != "" {
		statusCol = prefix + "status"
	}
	having = ""
	switch status {
	case "unused":
		having = "HAVING COUNT(*) FILTER (WHERE " + statusCol + " = 'unused') > 0"
	case "used":
		having = "HAVING COUNT(*) FILTER (WHERE " + statusCol + " = 'used') > 0"
	case "expired":
		having = "HAVING COUNT(*) FILTER (WHERE " + statusCol + " = 'expired') > 0"
	}
	_ = argN
	return strings.Join(parts, " AND "), having, args
}

// ListCodesByBatchKey returns all codes for a generation batch or a legacy singleton.
func (r *redeemCodeRepository) ListCodesByBatchKey(ctx context.Context, batchKey string) ([]service.RedeemCode, error) {
	client := clientFromContext(ctx, r.client)
	batchKey = strings.TrimSpace(batchKey)
	if batchKey == "" {
		return nil, fmt.Errorf("batch key is required")
	}

	q := client.RedeemCode.Query().WithUser().WithGroup()
	if strings.HasPrefix(batchKey, redeemLegacyBatchPrefix) {
		idStr := strings.TrimPrefix(batchKey, redeemLegacyBatchPrefix)
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid legacy batch key")
		}
		q = q.Where(redeemcode.IDEQ(id))
	} else {
		q = q.Where(redeemcode.BatchIDEQ(batchKey))
	}

	codes, err := q.Order(dbent.Asc(redeemcode.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return redeemCodeEntitiesToService(codes), nil
}

// DeleteUnusedByBatchKey deletes unused (and non-used) codes in a batch. Used codes are skipped.
func (r *redeemCodeRepository) DeleteUnusedByBatchKey(ctx context.Context, batchKey string) (int64, error) {
	client := clientFromContext(ctx, r.client)
	batchKey = strings.TrimSpace(batchKey)
	if batchKey == "" {
		return 0, fmt.Errorf("batch key is required")
	}

	del := client.RedeemCode.Delete().Where(redeemcode.StatusNEQ(service.StatusUsed))
	if strings.HasPrefix(batchKey, redeemLegacyBatchPrefix) {
		idStr := strings.TrimPrefix(batchKey, redeemLegacyBatchPrefix)
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid legacy batch key")
		}
		del = del.Where(redeemcode.IDEQ(id))
	} else {
		del = del.Where(redeemcode.BatchIDEQ(batchKey))
	}

	n, err := del.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func redeemCodeEntitiesToService(models []*dbent.RedeemCode) []service.RedeemCode {
	out := make([]service.RedeemCode, 0, len(models))
	for i := range models {
		if s := redeemCodeEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}
