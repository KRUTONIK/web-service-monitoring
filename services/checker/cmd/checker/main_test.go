package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

type stubWriter struct {
	result check.Result
}

func (stub *stubWriter) Write(_ context.Context, result check.Result) error {
	stub.result = result
	return nil
}

func TestRunJobWritesCheckResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writer := &stubWriter{}
	scheduledAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	job := jobs.NewCheckJob("service-1", server.URL, scheduledAt)
	if err := runJob(context.Background(), check.New(server.Client()), writer, job); err != nil {
		t.Fatalf("run check job: %v", err)
	}
	if writer.result.JobID != job.JobID || !writer.result.CheckedAt.Equal(scheduledAt) || writer.result.ServiceID != "service-1" || !writer.result.Available || writer.result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected result: %+v", writer.result)
	}
}
