package middleware

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"paygate/internal/audit"
	"paygate/internal/entity"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type mockAuditRepo struct {
	mu          sync.Mutex
	inboundLogs []*entity.InboundRequest
}

func (m *mockAuditRepo) SaveInbound(ctx context.Context, log *entity.InboundRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inboundLogs = append(m.inboundLogs, log)
	return nil
}

func (m *mockAuditRepo) SaveOutbound(ctx context.Context, log *entity.OutboundRequest) error {
	return nil
}

func TestAuditMiddleware_FilterBotScanners(t *testing.T) {
	repo := &mockAuditRepo{}
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	worker := audit.NewWorker(repo, log, 100)
	defer worker.Stop(context.Background())

	app := fiber.New()
	app.Use(NewAuditMiddleware(worker))

	app.Get("/health/live", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Post("/api/v1/payments/create", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// 1. Bot probes (non-/api/ paths) -> must NOT be recorded
	botPaths := []string{
		"/.env",
		"/var/www/html/.env",
		"/error_log",
		"/key.pem",
		"/health/live",
	}

	for _, path := range botPaths {
		req := httptest.NewRequest("GET", path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to execute request %s: %v", path, err)
		}
		if path == "/health/live" {
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("expected 200 for health check, got %d", resp.StatusCode)
			}
		} else {
			if resp.StatusCode != fiber.StatusNotFound {
				t.Errorf("expected 404 for unrouted path %s, got %d", path, resp.StatusCode)
			}
		}
	}

	// 2. Official /api/ paths -> MUST be recorded
	apiPaths := []string{
		"/api/v1/payments/create",
		"/api/v1/unknown-endpoint", // typo by merchant, still under /api/
	}

	for _, path := range apiPaths {
		req := httptest.NewRequest("POST", path, nil)
		_, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to execute request %s: %v", path, err)
		}
	}

	// Wait briefly for worker queue processing
	time.Sleep(50 * time.Millisecond)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Only the 2 /api/ requests should be captured
	if len(repo.inboundLogs) != 2 {
		t.Fatalf("expected 2 audit logs recorded, got %d", len(repo.inboundLogs))
	}

	for _, logged := range repo.inboundLogs {
		if logged.Endpoint != "/api/v1/payments/create" && logged.Endpoint != "/api/v1/unknown-endpoint" {
			t.Errorf("unexpected logged endpoint: %s", logged.Endpoint)
		}
	}
}
