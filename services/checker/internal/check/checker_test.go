package check

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Millisecond)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := New(server.Client()).Run(context.Background(), "service-1", server.URL)

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
	if result.ServiceID != "service-1" {
		t.Fatalf("unexpected service ID: %s", result.ServiceID)
	}
	if result.CheckedAt.IsZero() {
		t.Fatal("expected check timestamp to be set")
	}
	if result.ResponseTimeMS <= 0 {
		t.Fatalf("expected measured response time, got %d ms", result.ResponseTimeMS)
	}
}

func TestRunUnavailableStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := New(server.Client()).Run(context.Background(), "service-1", server.URL)

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
	result := New(client).Run(context.Background(), "service-1", server.URL)

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
	result := New(&http.Client{}).Run(context.Background(), "service-1", "://invalid")

	if result.Available {
		t.Fatal("expected invalid address to be unavailable")
	}
	if result.Error == "" {
		t.Fatal("expected invalid address error")
	}
}

func TestRunNetworkError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("open local listener: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close local listener: %v", err)
	}

	result := New(&http.Client{Timeout: time.Second}).Run(context.Background(), "service-1", "http://"+address)

	if result.Available {
		t.Fatal("expected unavailable service after network error")
	}
	if result.StatusCode != 0 {
		t.Fatalf("expected no HTTP status, got %d", result.StatusCode)
	}
	if result.Error == "" {
		t.Fatal("expected network error")
	}
}

func TestRunRejectsUnsupportedScheme(t *testing.T) {
	result := New(&http.Client{}).Run(context.Background(), "service-1", "ftp://example.com/file")

	if result.Available {
		t.Fatal("expected unsupported scheme to be unavailable")
	}
	if !strings.Contains(result.Error, ErrUnsupportedScheme.Error()) {
		t.Fatalf("expected unsupported scheme error, got %q", result.Error)
	}
}

func TestRunRejectsAddressWithoutHost(t *testing.T) {
	result := New(&http.Client{}).Run(context.Background(), "service-1", "http:")

	if result.Available {
		t.Fatal("expected address without host to be unavailable")
	}
	if result.Error != ErrMissingHost.Error() {
		t.Fatalf("expected missing host error, got %q", result.Error)
	}
}

func TestRunRejectsMissingServiceID(t *testing.T) {
	result := New(&http.Client{}).Run(context.Background(), "", "https://service.test")

	if result.Available {
		t.Fatal("expected result without service ID to be unavailable")
	}
	if result.Error != ErrMissingServiceID.Error() {
		t.Fatalf("expected missing service ID error, got %q", result.Error)
	}
}
