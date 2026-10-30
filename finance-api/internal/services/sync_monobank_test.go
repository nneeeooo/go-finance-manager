package services_test

import (
	"encoding/json"
	"finance-api/internal/models"
	"finance-api/internal/services"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMonobankApiClientBuilderAndFetch(t *testing.T) {
	// Mock Monobank HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/personal/statement/test-acc/1000/2000" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]models.MonobankStatement{
				{
					ID:          "mono-tx-1",
					Time:        1500,
					Description: "Coffee shop",
					Amount:      -6500,
				},
			})
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer mockServer.Close()

	client := services.MonobankApiClientBuilder(mockServer.URL, "test-token")
	if client == nil {
		t.Fatal("expected non-nil resty client")
	}

	// 1. Successful fetch
	statements, err := services.FetchMonobankStatementWindow(client, "test-acc", 1000, 2000)
	if err != nil {
		t.Fatalf("unexpected error fetching statements: %v", err)
	}

	if len(statements) != 1 || statements[0].ID != "mono-tx-1" {
		t.Fatalf("expected 1 statement with ID mono-tx-1, got: %v", statements)
	}

	// 2. Failed fetch (bad params)
	_, err = services.FetchMonobankStatementWindow(client, "bad-acc", 0, 0)
	if err == nil {
		t.Fatal("expected error for non-success response, got nil")
	}
}
