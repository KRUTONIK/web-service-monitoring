package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := New(server.Client()).Run(context.Background(), server.URL)

	if !result.Available {
		t.Fatal("expected service to be available")
	}
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", result.StatusCode)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.ServiceURL != server.URL {
		t.Fatalf("unexpected service URL: %s", result.ServiceURL)
	}
	if result.CheckedAt.IsZero() {
		t.Fatal("expected check timestamp to be set")
	}
}

func TestRunUnavailableStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := New(server.Client()).Run(context.Background(), server.URL)

	if result.Available {
		t.Fatal("expected service to be unavailable")
	}
	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: %d", result.StatusCode)
	}
	if result.Error != "" {
		t.Fatalf("unexpected transport error: %s", result.Error)
	}
}

func TestRunTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 10 * time.Millisecond
	result := New(client).Run(context.Background(), server.URL)

	if result.Available {
		t.Fatal("expected timed out service to be unavailable")
	}
	if result.StatusCode != 0 {
		t.Fatalf("expected no HTTP status, got %d", result.StatusCode)
	}
	if result.Error == "" {
		t.Fatal("expected timeout error")
	}
}

func TestRunInvalidAddress(t *testing.T) {
	result := New(&http.Client{}).Run(context.Background(), "://invalid")

	if result.Available {
		t.Fatal("expected invalid address to be unavailable")
	}
	if result.Error == "" {
		t.Fatal("expected invalid address error")
	}
}
