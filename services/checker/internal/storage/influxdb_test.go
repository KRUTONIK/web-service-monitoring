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

	result := check.Result{JobID: "job-1", ServiceID: "service-1", ServiceURL: "https://service.test/path?a=b,c", CheckedAt: time.Unix(123, 456).UTC(), Available: false, StatusCode: 503, ResponseTimeMS: 27, Error: `request failed: "temporary"`}
	if err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Write(context.Background(), result); err != nil {
		t.Fatalf("write result: %v", err)
	}
	expected := `service_check,job_id=job-1,service_id=service-1,service_url=https://service.test/path?a\=b\,c available=false,status_code=503i,response_time_ms=27i,error="request failed: \"temporary\"" 123000000456`
	if receivedBody != expected {
		t.Fatalf("unexpected line protocol:\nwant: %s\ngot:  %s", expected, receivedBody)
	}
}

func TestWriteReturnsInfluxDBError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("invalid token"))
	}))
	defer server.Close()
	err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "bad-token").Write(context.Background(), check.Result{CheckedAt: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request for permanent error, got %d", requests)
	}
}

func TestWriteRetriesTemporaryInfluxDBError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := check.Result{ServiceID: "service-1", ServiceURL: "https://service.test", CheckedAt: time.Now().UTC()}
	if err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Write(context.Background(), result); err != nil {
		t.Fatalf("write result after retry: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected two write attempts, got %d", requests)
	}
}

func TestWriteStopsAfterRetryLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Write(context.Background(), check.Result{ServiceID: "service-1", CheckedAt: time.Now().UTC()})
	if err == nil || !strings.Contains(err.Error(), "429 Too Many Requests") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != maxWriteAttempts {
		t.Fatalf("expected %d write attempts, got %d", maxWriteAttempts, requests)
	}
}

func TestRepeatedWriteUsesSameInfluxDBPoint(t *testing.T) {
	bodies := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		bodies = append(bodies, string(body))
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := check.Result{JobID: "job-1", ServiceID: "service-1", ServiceURL: "https://service.test", CheckedAt: time.Unix(123, 456).UTC(), Available: true, StatusCode: http.StatusOK, ResponseTimeMS: 12}
	repeatedResult := result
	repeatedResult.Available = false
	repeatedResult.StatusCode = http.StatusServiceUnavailable
	storage := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token")
	if err := storage.Write(context.Background(), result); err != nil {
		t.Fatalf("write first result: %v", err)
	}
	if err := storage.Write(context.Background(), repeatedResult); err != nil {
		t.Fatalf("write repeated result: %v", err)
	}
	if len(bodies) != 2 || pointIdentity(bodies[0]) != pointIdentity(bodies[1]) {
		t.Fatalf("expected repeated delivery to use the same point, got %v", bodies)
	}
}

func pointIdentity(line string) string {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return ""
	}
	return parts[0] + " " + parts[2]
}
