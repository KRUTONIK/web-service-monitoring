package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

func TestWriteCheckResult(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", request.Method)
		}
		if request.URL.Path != "/api/v2/write" {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("org") != "test-org" {
			t.Errorf("unexpected organization: %s", request.URL.Query().Get("org"))
		}
		if request.URL.Query().Get("bucket") != "test-bucket" {
			t.Errorf("unexpected bucket: %s", request.URL.Query().Get("bucket"))
		}
		if request.URL.Query().Get("precision") != "ns" {
			t.Errorf("unexpected precision: %s", request.URL.Query().Get("precision"))
		}
		if request.Header.Get("Authorization") != "Token test-token" {
			t.Errorf("unexpected authorization header: %s", request.Header.Get("Authorization"))
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		receivedBody = string(body)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := check.Result{
		ServiceURL:     "https://service.test/path?a=b,c",
		CheckedAt:      time.Unix(123, 456).UTC(),
		Available:      false,
		StatusCode:     http.StatusServiceUnavailable,
		ResponseTimeMS: 27,
		Error:          `request failed: "temporary"`,
	}
	influx := NewInfluxDB(server.Client(), server.URL, "test-org", "test-bucket", "test-token")

	if err := influx.Write(context.Background(), result); err != nil {
		t.Fatalf("write result: %v", err)
	}

	expected := `service_check,service_url=https://service.test/path?a\=b\,c available=false,status_code=503i,response_time_ms=27i,error="request failed: \"temporary\"" 123000000456`
	if receivedBody != expected {
		t.Fatalf("unexpected line protocol:\nwant: %s\ngot:  %s", expected, receivedBody)
	}
}

func TestWriteReturnsInfluxDBError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("invalid token"))
	}))
	defer server.Close()

	influx := NewInfluxDB(server.Client(), server.URL, "test-org", "test-bucket", "bad-token")
	err := influx.Write(context.Background(), check.Result{CheckedAt: time.Now()})

	if err == nil {
		t.Fatal("expected InfluxDB error")
	}
	if !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("expected response body in error: %v", err)
	}
}
