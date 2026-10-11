package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

const measurementName = "service_check"

const (
	maxWriteAttempts = 3
	retryBaseDelay   = 50 * time.Millisecond
)

type InfluxDB struct {
	client  *http.Client
	baseURL string
	org     string
	bucket  string
	token   string
}

func NewInfluxDB(client *http.Client, baseURL string, org string, bucket string, token string) *InfluxDB {
	return &InfluxDB{client: client, baseURL: strings.TrimRight(baseURL, "/"), org: org, bucket: bucket, token: token}
}

func (influx *InfluxDB) Write(ctx context.Context, result check.Result) error {
	for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
		retryable, err := influx.writeOnce(ctx, result)
		if err == nil {
			return nil
		}
		if !retryable || attempt == maxWriteAttempts {
			return err
		}

		delay := retryBaseDelay * time.Duration(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil
}

func (influx *InfluxDB) writeOnce(ctx context.Context, result check.Result) (bool, error) {
	endpoint, err := url.Parse(influx.baseURL + "/api/v2/write")
	if err != nil {
		return false, fmt.Errorf("parse InfluxDB URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("org", influx.org)
	query.Set("bucket", influx.bucket)
	query.Set("precision", "ns")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewBufferString(lineProtocol(result)))
	if err != nil {
		return false, fmt.Errorf("create InfluxDB request: %w", err)
	}
	request.Header.Set("Authorization", "Token "+influx.token)
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	response, err := influx.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return true, fmt.Errorf("write result to InfluxDB: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError,
			fmt.Errorf("InfluxDB returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return false, nil
}

func lineProtocol(result check.Result) string {
	fields := []string{
		fmt.Sprintf("available=%t", result.Available),
		fmt.Sprintf("status_code=%di", result.StatusCode),
		fmt.Sprintf("response_time_ms=%di", result.ResponseTimeMS),
	}
	if result.Error != "" {
		fields = append(fields, fmt.Sprintf("error=\"%s\"", escapeFieldString(result.Error)))
	}
	return fmt.Sprintf("%s,job_id=%s,service_id=%s,service_url=%s %s %d", measurementName, escapeTag(result.JobID), escapeTag(result.ServiceID), escapeTag(result.ServiceURL), strings.Join(fields, ","), result.CheckedAt.UnixNano())
}

func escapeTag(value string) string {
	return strings.NewReplacer("\\", "\\\\", ",", "\\,", " ", "\\ ", "=", "\\=").Replace(value)
}

func escapeFieldString(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value)
}
