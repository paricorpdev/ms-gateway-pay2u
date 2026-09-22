package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"paygate/internal/common/generator"
	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
	"paygate/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MerchantUseCase struct {
	db           *gorm.DB
	merchantRepo repository.MerchantRepository
	cache        repository.CacheRepository
}

func NewMerchantUseCase(
	db *gorm.DB,
	merchantRepo repository.MerchantRepository,
	cache repository.CacheRepository,
) *MerchantUseCase {
	return &MerchantUseCase{
		db:           db,
		merchantRepo: merchantRepo,
		cache:        cache,
	}
}

func (u *MerchantUseCase) cacheKey(apiKey string) string {
	return "paygate:merchant:apikey:" + apiKey
}

func (u *MerchantUseCase) cacheMerchant(ctx context.Context, m *entity.Merchant) {
	if u.cache == nil || m == nil || m.APIKey == "" {
		return
	}
	bytes, err := json.Marshal(m)
	if err == nil {
		_ = u.cache.Set(ctx, u.cacheKey(m.APIKey), bytes, 1*time.Hour)
	}
}

func (u *MerchantUseCase) CreateMerchant(ctx context.Context, req *payload.CreateMerchantRequest) (*payload.MerchantResponse, error) {
	code := req.Code
	if code == "" {
		code = generator.NormalizeCode(req.Name)
	} else {
		code = generator.NormalizeCode(code)
	}

	if code == "" {
		return nil, exception.BadRequest("merchant code cannot be empty")
	}

	existing, err := u.merchantRepo.FindByCode(ctx, u.db, code)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("check existing code: %w", err))
	}
	if existing != nil {
		return nil, exception.Conflict(fmt.Sprintf("merchant with code %q already exists", code))
	}

	apiKey, err := generator.GenerateAPIKey(code)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("generate api key: %w", err))
	}

	webhookSecret, err := generator.GenerateWebhookSecret()
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("generate webhook secret: %w", err))
	}

	now := time.Now().UTC()
	merchant := &entity.Merchant{
		ID:            uuid.New(),
		Code:          code,
		Name:          req.Name,
		APIKey:        apiKey,
		WebhookURL:    req.WebhookURL,
		WebhookSecret: webhookSecret,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := u.merchantRepo.Create(ctx, u.db, merchant); err != nil {
		return nil, exception.Internal(fmt.Errorf("save merchant: %w", err))
	}

	u.cacheMerchant(ctx, merchant)

	return u.toMerchantResponse(merchant), nil
}

func (u *MerchantUseCase) GetMerchant(ctx context.Context, id uuid.UUID) (*payload.MerchantResponse, error) {
	m, err := u.merchantRepo.FindByID(ctx, u.db, id)
	if err != nil {
		return nil, err
	}
	return u.toMerchantResponse(m), nil
}

func (u *MerchantUseCase) ListMerchants(ctx context.Context, req payload.PageRequest) (*payload.PageResponse[*payload.MerchantResponse], error) {
	req.Normalize()

	merchants, total, err := u.merchantRepo.FindAll(ctx, u.db, req.PerPage, req.Offset())
	if err != nil {
		return nil, exception.Internal(err)
	}

	responses := make([]*payload.MerchantResponse, 0, len(merchants))
	for _, m := range merchants {
		responses = append(responses, u.toMerchantResponse(m))
	}

	return payload.NewPageResponse(responses, req.Page, req.PerPage, total), nil
}

func (u *MerchantUseCase) UpdateMerchant(ctx context.Context, id uuid.UUID, req *payload.UpdateMerchantRequest) (*payload.MerchantResponse, error) {
	m, err := u.merchantRepo.FindByID(ctx, u.db, id)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]any)
	if req.Name != nil && *req.Name != "" {
		updates["name"] = *req.Name
	}
	if req.WebhookURL != nil {
		updates["webhook_url"] = *req.WebhookURL
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	updates["updated_at"] = time.Now().UTC()

	if err := u.merchantRepo.Update(ctx, u.db, id, updates); err != nil {
		return nil, err
	}

	if u.cache != nil {
		_ = u.cache.Delete(ctx, u.cacheKey(m.APIKey))
	}

	updated, err := u.merchantRepo.FindByID(ctx, u.db, id)
	if err != nil {
		return nil, err
	}

	if updated.IsActive {
		u.cacheMerchant(ctx, updated)
	}

	return u.toMerchantResponse(updated), nil
}

func (u *MerchantUseCase) RotateAPIKey(ctx context.Context, id uuid.UUID) (*payload.MerchantResponse, error) {
	m, err := u.merchantRepo.FindByID(ctx, u.db, id)
	if err != nil {
		return nil, err
	}

	if u.cache != nil {
		_ = u.cache.Delete(ctx, u.cacheKey(m.APIKey))
	}

	newKey, err := generator.GenerateAPIKey(m.Code)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("generate api key: %w", err))
	}

	now := time.Now().UTC()
	updates := map[string]any{
		"api_key":    newKey,
		"updated_at": now,
	}

	if err := u.merchantRepo.Update(ctx, u.db, id, updates); err != nil {
		return nil, err
	}

	m.APIKey = newKey
	m.UpdatedAt = now

	if m.IsActive {
		u.cacheMerchant(ctx, m)
	}

	return u.toMerchantResponse(m), nil
}

func (u *MerchantUseCase) ValidateAPIKey(ctx context.Context, apiKey string) (*entity.Merchant, error) {
	if apiKey == "" {
		return nil, exception.Unauthorized("missing API key")
	}

	if u.cache != nil {
		if cached, err := u.cache.Get(ctx, u.cacheKey(apiKey)); err == nil && len(cached) > 0 {
			var m entity.Merchant
			if err := json.Unmarshal(cached, &m); err == nil {
				if !m.IsActive {
					return nil, exception.Unauthorized("merchant is inactive")
				}
				return &m, nil
			}
		}
	}

	m, err := u.merchantRepo.FindByAPIKey(ctx, u.db, apiKey)
	if err != nil {
		return nil, exception.Internal(err)
	}
	if m == nil {
		return nil, exception.Unauthorized("invalid API key")
	}
	if !m.IsActive {
		return nil, exception.Unauthorized("merchant is inactive")
	}

	u.cacheMerchant(ctx, m)

	return m, nil
}

func (u *MerchantUseCase) toMerchantResponse(m *entity.Merchant) *payload.MerchantResponse {
	return &payload.MerchantResponse{
		ID:            m.ID,
		Code:          m.Code,
		Name:          m.Name,
		APIKey:        m.APIKey,
		WebhookURL:    m.WebhookURL,
		WebhookSecret: m.WebhookSecret,
		IsActive:      m.IsActive,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}
