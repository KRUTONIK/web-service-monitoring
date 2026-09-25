package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/storage"
)

type stubCheckReader struct {
	result storage.CheckResult
	err    error
}

func (stub stubCheckReader) Latest(context.Context) (storage.CheckResult, error) {
	return stub.result, stub.err
}

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	New(stubCheckReader{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestLatestCheck(t *testing.T) {
	checkedAt := time.Date(2026, time.September, 25, 10, 30, 0, 0, time.UTC)
	reader := stubCheckReader{result: storage.CheckResult{
		ServiceURL:     "https://service.test",
		CheckedAt:      checkedAt,
		Available:      true,
		StatusCode:     http.StatusOK,
		ResponseTimeMS: 42,
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/checks/latest", nil)
	response := httptest.NewRecorder()

	New(reader).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"service_url":"https://service.test"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS header: %s", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestLatestCheckNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/checks/latest", nil)
	response := httptest.NewRecorder()

	New(stubCheckReader{err: storage.ErrNotFound}).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestLatestCheckStorageFailure(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/checks/latest", nil)
	response := httptest.NewRecorder()

	New(stubCheckReader{err: errors.New("storage unavailable")}).ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}
