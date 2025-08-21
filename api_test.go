package main

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/stretchr/testify/assert"
)

// Test Rate Limiting
func TestRateLimiting(t *testing.T) {
	app := fiber.New()

	app.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			return "test-ip"
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
	}))

	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// First request - should succeed
	req := httptest.NewRequest("GET", "/test", nil)

	for i := 0; i < 10; i++ {
		resp, _ := app.Test(req)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	//11th request - should fail
	resp, _ := app.Test(req)
	assert.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	time.Sleep(1 * time.Minute)

	//final request - should succeed
	resp, _ = app.Test(req)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test Authentication
func TestAuthentication(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware)
	
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Setup test API key
	apiKeys = map[string]struct{}{
		"test-key": {},
	}

	// Test valid API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "test-key")
	resp, _ := app.Test(req)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Test invalid API key
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "invalid-key")
	resp, _ = app.Test(req)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	// Test missing API key
	req = httptest.NewRequest("GET", "/test", nil)
	resp, _ = app.Test(req)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// Test Cache
func TestCache(t *testing.T) {
	balance := &Balance{
		Wallet:    "test-wallet",
		Amount:    100.0,
		FetchedAt: time.Now().Unix(),
	}

	// Test cache set
	err := SetBalanceInCache(balance)
	assert.NoError(t, err)

	// Test cache get
	cached, err := GetBalanceFromCache("test-wallet")
	assert.NoError(t, err)
	if cached != nil {
		assert.Equal(t, balance.Amount, cached.Amount)
	}

	//Test 10s TTL
	time.Sleep(10 * time.Second)
	cached, err = GetBalanceFromCache("test-wallet")
	assert.NoError(t, err)
	assert.Nil(t, cached)
}
