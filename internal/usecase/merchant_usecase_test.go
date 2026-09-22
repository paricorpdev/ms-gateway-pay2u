package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/model/payload"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type inMemoryMerchantRepo struct {
	mu        sync.RWMutex
	merchants map[uuid.UUID]*entity.Merchant
}

func newInMemoryMerchantRepo() *inMemoryMerchantRepo {
	return &inMemoryMerchantRepo{
		merchants: make(map[uuid.UUID]*entity.Merchant),
	}
}

func (r *inMemoryMerchantRepo) Create(ctx context.Context, db *gorm.DB, m *entity.Merchant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.merchants[m.ID] = m
	return nil
}

func (r *inMemoryMerchantRepo) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Merchant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.merchants[id]
	if !ok {
		return nil, exception.NotFound("merchant not found")
	}
	return m, nil
}

func (r *inMemoryMerchantRepo) FindByAPIKey(ctx context.Context, db *gorm.DB, key string) (*entity.Merchant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.merchants {
		if m.APIKey == key {
			return m, nil
		}
	}
	return nil, nil
}

func (r *inMemoryMerchantRepo) FindByCode(ctx context.Context, db *gorm.DB, code string) (*entity.Merchant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.merchants {
		if m.Code == code {
			return m, nil
		}
	}
	return nil, nil
}

func (r *inMemoryMerchantRepo) FindAll(ctx context.Context, db *gorm.DB, limit, offset int) ([]*entity.Merchant, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*entity.Merchant
	for _, m := range r.merchants {
		list = append(list, m)
	}
	return list, int64(len(list)), nil
}

func (r *inMemoryMerchantRepo) Update(ctx context.Context, db *gorm.DB, id uuid.UUID, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.merchants[id]
	if !ok {
		return exception.NotFound("merchant not found")
	}
	if v, ok := updates["name"].(string); ok {
		m.Name = v
	}
	if v, ok := updates["api_key"].(string); ok {
		m.APIKey = v
	}
	if v, ok := updates["webhook_url"].(string); ok {
		m.WebhookURL = v
	}
	if v, ok := updates["is_active"].(bool); ok {
		m.IsActive = v
	}
	return nil
}

type inMemoryCache struct {
	mu       sync.RWMutex
	store    map[string][]byte
	getCalls int
}

func newInMemoryCache() *inMemoryCache {
	return &inMemoryCache{store: make(map[string][]byte)}
}

func (c *inMemoryCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = val
	return nil
}

func (c *inMemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCalls++
	val, ok := c.store[key]
	if !ok {
		return nil, exception.NotFound("cache miss")
	}
	return val, nil
}

func (c *inMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
	return nil
}

func TestMerchantUseCase_CreateAndValidate(t *testing.T) {
	repo := newInMemoryMerchantRepo()
	cache := newInMemoryCache()
	uc := NewMerchantUseCase(nil, repo, cache)

	ctx := context.Background()

	res, err := uc.CreateMerchant(ctx, &payload.CreateMerchantRequest{
		Name: "Localoka V2",
	})
	if err != nil {
		t.Fatalf("unexpected error creating merchant: %v", err)
	}

	if res.Code != "localoka-v2" {
		t.Errorf("expected code 'localoka-v2', got %q", res.Code)
	}
	if !strings.HasPrefix(res.APIKey, "pg_localoka-v2_") {
		t.Errorf("expected API key starting with 'pg_localoka-v2_', got %q", res.APIKey)
	}
	if !strings.HasPrefix(res.WebhookSecret, "whsec_") {
		t.Errorf("expected webhook secret starting with 'whsec_', got %q", res.WebhookSecret)
	}

	// Validate API Key - should be in cache from CreateMerchant
	m, err := uc.ValidateAPIKey(ctx, res.APIKey)
	if err != nil {
		t.Fatalf("unexpected error validating api key: %v", err)
	}
	if m.ID != res.ID {
		t.Errorf("expected merchant ID %v, got %v", res.ID, m.ID)
	}

	// Delete from cache to simulate cold lookup (DB fallback)
	_ = cache.Delete(ctx, uc.cacheKey(res.APIKey))

	mCold, err := uc.ValidateAPIKey(ctx, res.APIKey)
	if err != nil {
		t.Fatalf("unexpected error on cold validate: %v", err)
	}
	if mCold.Code != "localoka-v2" {
		t.Errorf("expected cold merchant code 'localoka-v2', got %q", mCold.Code)
	}

	// Duplicate code should be rejected
	_, err = uc.CreateMerchant(ctx, &payload.CreateMerchantRequest{
		Name: "Localoka V2 Duplicate",
		Code: "localoka-v2",
	})
	if err == nil {
		t.Fatal("expected conflict error on duplicate code, got nil")
	}
}

func TestMerchantUseCase_RotateKey(t *testing.T) {
	repo := newInMemoryMerchantRepo()
	cache := newInMemoryCache()
	uc := NewMerchantUseCase(nil, repo, cache)
	ctx := context.Background()

	created, _ := uc.CreateMerchant(ctx, &payload.CreateMerchantRequest{
		Name: "Billing Service",
	})

	oldKey := created.APIKey

	rotated, err := uc.RotateAPIKey(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error rotating key: %v", err)
	}

	if rotated.APIKey == oldKey {
		t.Errorf("expected new key to be different from old key")
	}
	if !strings.HasPrefix(rotated.APIKey, "pg_billing-service_") {
		t.Errorf("expected new key with prefix 'pg_billing-service_', got %q", rotated.APIKey)
	}

	// Old key should now fail
	_, err = uc.ValidateAPIKey(ctx, oldKey)
	if err == nil {
		t.Error("expected old key to be invalid after rotation, but validate succeeded")
	}

	// New key should succeed
	m, err := uc.ValidateAPIKey(ctx, rotated.APIKey)
	if err != nil || m == nil {
		t.Errorf("expected new key to validate successfully, got err=%v", err)
	}
}

func TestMerchantUseCase_InactiveMerchant(t *testing.T) {
	repo := newInMemoryMerchantRepo()
	cache := newInMemoryCache()
	uc := NewMerchantUseCase(nil, repo, cache)
	ctx := context.Background()

	created, _ := uc.CreateMerchant(ctx, &payload.CreateMerchantRequest{
		Name: "Test Inactive",
	})

	activeFalse := false
	_, err := uc.UpdateMerchant(ctx, created.ID, &payload.UpdateMerchantRequest{
		IsActive: &activeFalse,
	})
	if err != nil {
		t.Fatalf("unexpected error updating merchant: %v", err)
	}

	_, err = uc.ValidateAPIKey(ctx, created.APIKey)
	if err == nil {
		t.Fatal("expected inactive merchant to be rejected, got nil")
	}
}

func TestMerchantUseCase_ListMerchants(t *testing.T) {
	repo := newInMemoryMerchantRepo()
	cache := newInMemoryCache()
	uc := NewMerchantUseCase(nil, repo, cache)
	ctx := context.Background()

	_, _ = uc.CreateMerchant(ctx, &payload.CreateMerchantRequest{Name: "Merchant Alpha"})
	_, _ = uc.CreateMerchant(ctx, &payload.CreateMerchantRequest{Name: "Merchant Beta"})

	// Default pagination
	res, err := uc.ListMerchants(ctx, payload.PageRequest{})
	if err != nil {
		t.Fatalf("unexpected error listing merchants: %v", err)
	}

	if len(res.Data) != 2 {
		t.Errorf("expected 2 merchants, got %d", len(res.Data))
	}
	if res.PageMeta.Page != payload.DefaultPage {
		t.Errorf("expected page %d, got %d", payload.DefaultPage, res.PageMeta.Page)
	}
	if res.PageMeta.PerPage != payload.DefaultPerPage {
		t.Errorf("expected per_page %d, got %d", payload.DefaultPerPage, res.PageMeta.PerPage)
	}
	if res.PageMeta.TotalData != 2 {
		t.Errorf("expected total_data 2, got %d", res.PageMeta.TotalData)
	}
	if res.PageMeta.TotalPage != 1 {
		t.Errorf("expected total_page 1, got %d", res.PageMeta.TotalPage)
	}
}

