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
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		receivedBody = string(body)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := check.Result{ServiceURL: "https://service.test/path?a=b,c", CheckedAt: time.Unix(123, 456).UTC(), Available: false, StatusCode: 503, ResponseTimeMS: 27, Error: `request failed: "temporary"`}
	if err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Write(context.Background(), result); err != nil {
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
	err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "bad-token").Write(context.Background(), check.Result{CheckedAt: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("unexpected error: %v", err)
	}
}
