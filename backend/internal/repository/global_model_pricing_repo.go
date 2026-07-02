package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type globalModelPricingRepository struct {
	db *sql.DB
}

const globalModelPricingSelectColumns = `id, model, provider, billing_mode, input_price, output_price, cache_write_price, cache_read_price, image_output_price, per_request_price, image_price_1k, image_price_2k, image_price_4k, image_billing_strategy, image_megapixel_price, image_quality_prices, image_quality_multipliers, image_tier_rules, enabled, notes, display_input_price, display_output_price, display_cache_read_price, display_cache_creation_price, display_rate_multiplier, show_on_pricing_page, created_at, updated_at`

// NewGlobalModelPricingRepository 创建全局模型定价数据访问实例
func NewGlobalModelPricingRepository(db *sql.DB) service.GlobalModelPricingRepository {
	return &globalModelPricingRepository{db: db}
}

func (r *globalModelPricingRepository) List(ctx context.Context, params pagination.PaginationParams, search, provider string) ([]service.GlobalModelPricing, *pagination.PaginationResult, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(model) LIKE $%d", argIdx))
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}
	if provider != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(provider) = $%d", argIdx))
		args = append(args, strings.ToLower(provider))
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM global_model_pricing %s", where)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count global model pricing: %w", err)
	}

	// Paginated query
	sortBy := "model"
	if params.SortBy != "" {
		allowedSorts := map[string]bool{"model": true, "provider": true, "created_at": true, "updated_at": true, "enabled": true}
		if allowedSorts[params.SortBy] {
			sortBy = params.SortBy
		}
	}
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderAsc)

	query := fmt.Sprintf(
		`SELECT `+globalModelPricingSelectColumns+`
		 FROM global_model_pricing %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, sortBy, sortOrder, argIdx, argIdx+1,
	)
	args = append(args, params.Limit(), params.Offset())

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list global model pricing: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []service.GlobalModelPricing
	for rows.Next() {
		p, err := scanGlobalModelPricing(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan global model pricing: %w", err)
		}
		result = append(result, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate global model pricing: %w", err)
	}

	pages := int(total) / params.Limit()
	if int(total)%params.Limit() > 0 {
		pages++
	}

	return result, &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: params.Limit(),
		Pages:    pages,
	}, nil
}

func (r *globalModelPricingRepository) GetByID(ctx context.Context, id int64) (*service.GlobalModelPricing, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+globalModelPricingSelectColumns+`
		 FROM global_model_pricing WHERE id = $1`, id,
	)
	p, err := scanGlobalModelPricing(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get global model pricing by id: %w", err)
	}
	return p, nil
}

func (r *globalModelPricingRepository) GetByModel(ctx context.Context, model string) (*service.GlobalModelPricing, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+globalModelPricingSelectColumns+`
		 FROM global_model_pricing WHERE LOWER(model) = $1`, strings.ToLower(model),
	)
	p, err := scanGlobalModelPricing(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get global model pricing by model: %w", err)
	}
	return p, nil
}

func (r *globalModelPricingRepository) Create(ctx context.Context, pricing *service.GlobalModelPricing) error {
	billingMode := pricing.BillingMode
	if billingMode == "" {
		billingMode = service.BillingModeToken
	}
	imageStrategy := service.NormalizeImageBillingStrategy(pricing.ImageBillingStrategy)
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO global_model_pricing (model, provider, billing_mode, input_price, output_price, cache_write_price, cache_read_price, image_output_price, per_request_price, image_price_1k, image_price_2k, image_price_4k, image_billing_strategy, image_megapixel_price, image_quality_prices, image_quality_multipliers, image_tier_rules, enabled, notes, display_input_price, display_output_price, display_cache_read_price, display_cache_creation_price, display_rate_multiplier, show_on_pricing_page)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		 RETURNING id, created_at, updated_at`,
		pricing.Model, pricing.Provider, billingMode,
		pricing.InputPrice, pricing.OutputPrice, pricing.CacheWritePrice, pricing.CacheReadPrice,
		pricing.ImageOutputPrice, pricing.PerRequestPrice,
		pricing.ImagePrice1K, pricing.ImagePrice2K, pricing.ImagePrice4K,
		imageStrategy, pricing.ImageMegapixelPrice, service.ImageQualityPricesJSON(pricing.ImageQualityPrices), service.ImageQualityMultipliersJSON(pricing.ImageQualityMultipliers), service.ImageTierRulesJSON(pricing.ImageTierRules),
		pricing.Enabled, pricing.Notes,
		pricing.DisplayInputPrice, pricing.DisplayOutputPrice, pricing.DisplayCacheReadPrice, pricing.DisplayCacheCreationPrice, pricing.DisplayRateMultiplier,
		pricing.ShowOnPricingPage,
	).Scan(&pricing.ID, &pricing.CreatedAt, &pricing.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("model %q already has a global pricing override", pricing.Model)
		}
		return fmt.Errorf("create global model pricing: %w", err)
	}
	return nil
}

func (r *globalModelPricingRepository) Update(ctx context.Context, pricing *service.GlobalModelPricing) error {
	billingMode := pricing.BillingMode
	if billingMode == "" {
		billingMode = service.BillingModeToken
	}
	imageStrategy := service.NormalizeImageBillingStrategy(pricing.ImageBillingStrategy)
	result, err := r.db.ExecContext(ctx,
		`UPDATE global_model_pricing
		 SET model = $1, provider = $2, billing_mode = $3, input_price = $4, output_price = $5, cache_write_price = $6, cache_read_price = $7, image_output_price = $8, per_request_price = $9, image_price_1k = $10, image_price_2k = $11, image_price_4k = $12, image_billing_strategy = $13, image_megapixel_price = $14, image_quality_prices = $15, image_quality_multipliers = $16, image_tier_rules = $17, enabled = $18, notes = $19, display_input_price = $20, display_output_price = $21, display_cache_read_price = $22, display_cache_creation_price = $23, display_rate_multiplier = $24, show_on_pricing_page = $25, updated_at = NOW()
		 WHERE id = $26`,
		pricing.Model, pricing.Provider, billingMode,
		pricing.InputPrice, pricing.OutputPrice, pricing.CacheWritePrice, pricing.CacheReadPrice,
		pricing.ImageOutputPrice, pricing.PerRequestPrice,
		pricing.ImagePrice1K, pricing.ImagePrice2K, pricing.ImagePrice4K,
		imageStrategy, pricing.ImageMegapixelPrice, service.ImageQualityPricesJSON(pricing.ImageQualityPrices), service.ImageQualityMultipliersJSON(pricing.ImageQualityMultipliers), service.ImageTierRulesJSON(pricing.ImageTierRules),
		pricing.Enabled, pricing.Notes,
		pricing.DisplayInputPrice, pricing.DisplayOutputPrice, pricing.DisplayCacheReadPrice, pricing.DisplayCacheCreationPrice, pricing.DisplayRateMultiplier,
		pricing.ShowOnPricingPage,
		pricing.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("model %q already has a global pricing override", pricing.Model)
		}
		return fmt.Errorf("update global model pricing: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("global model pricing not found: %d", pricing.ID)
	}
	return nil
}

func (r *globalModelPricingRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM global_model_pricing WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete global model pricing: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("global model pricing not found: %d", id)
	}
	return nil
}

func (r *globalModelPricingRepository) GetAllEnabled(ctx context.Context) ([]service.GlobalModelPricing, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+globalModelPricingSelectColumns+`
		 FROM global_model_pricing WHERE enabled = true ORDER BY model`,
	)
	if err != nil {
		return nil, fmt.Errorf("get all enabled global model pricing: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []service.GlobalModelPricing
	for rows.Next() {
		p, err := scanGlobalModelPricing(rows)
		if err != nil {
			return nil, fmt.Errorf("scan global model pricing: %w", err)
		}
		result = append(result, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global model pricing: %w", err)
	}
	return result, nil
}

// ListForPricingPage 返回同时满足 enabled=true 且 show_on_pricing_page=true 的模型，
// 按 provider、model 升序排序，供用户侧「模型计价」页展示。
func (r *globalModelPricingRepository) ListForPricingPage(ctx context.Context) ([]service.GlobalModelPricing, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+globalModelPricingSelectColumns+`
		 FROM global_model_pricing
		 WHERE enabled = true AND show_on_pricing_page = true
		 ORDER BY provider ASC, model ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pricing-page models: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []service.GlobalModelPricing
	for rows.Next() {
		p, err := scanGlobalModelPricing(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pricing-page model: %w", err)
		}
		result = append(result, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pricing-page models: %w", err)
	}
	return result, nil
}

type globalPricingScanner interface {
	Scan(dest ...any) error
}

func scanGlobalModelPricing(scanner globalPricingScanner) (*service.GlobalModelPricing, error) {
	var p service.GlobalModelPricing
	var imageQualityPrices sql.NullString
	var imageQualityMultipliers sql.NullString
	var imageTierRules sql.NullString
	if err := scanner.Scan(
		&p.ID, &p.Model, &p.Provider, &p.BillingMode,
		&p.InputPrice, &p.OutputPrice, &p.CacheWritePrice, &p.CacheReadPrice,
		&p.ImageOutputPrice, &p.PerRequestPrice,
		&p.ImagePrice1K, &p.ImagePrice2K, &p.ImagePrice4K,
		&p.ImageBillingStrategy, &p.ImageMegapixelPrice, &imageQualityPrices, &imageQualityMultipliers, &imageTierRules,
		&p.Enabled, &p.Notes,
		&p.DisplayInputPrice, &p.DisplayOutputPrice, &p.DisplayCacheReadPrice, &p.DisplayCacheCreationPrice, &p.DisplayRateMultiplier,
		&p.ShowOnPricingPage,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	p.ImageBillingStrategy = service.NormalizeImageBillingStrategy(p.ImageBillingStrategy)
	if imageQualityPrices.Valid {
		p.ImageQualityPrices = service.ParseImageQualityPricesJSON(imageQualityPrices.String)
	}
	if imageQualityMultipliers.Valid {
		p.ImageQualityMultipliers = service.ParseImageQualityMultipliersJSON(imageQualityMultipliers.String)
	}
	if imageTierRules.Valid {
		p.ImageTierRules = service.ParseImageTierRulesJSON(imageTierRules.String)
	}
	return &p, nil
}
