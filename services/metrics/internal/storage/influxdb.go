package storage

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/metric"
)

var ErrNotFound = errors.New("check result not found")

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

func (influx *InfluxDB) Latest(ctx context.Context) (metric.Result, error) {
	endpoint, err := url.Parse(influx.baseURL + "/api/v2/query")
	if err != nil {
		return metric.Result{}, fmt.Errorf("parse InfluxDB URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("org", influx.org)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewBufferString(influx.latestQuery()))
	if err != nil {
		return metric.Result{}, fmt.Errorf("create InfluxDB query request: %w", err)
	}
	request.Header.Set("Authorization", "Token "+influx.token)
	request.Header.Set("Accept", "application/csv")
	request.Header.Set("Content-Type", "application/vnd.flux")

	response, err := influx.client.Do(request)
	if err != nil {
		return metric.Result{}, fmt.Errorf("query InfluxDB: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return metric.Result{}, fmt.Errorf("InfluxDB returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	result, err := parseLatestResult(response.Body)
	if err != nil {
		return metric.Result{}, fmt.Errorf("parse InfluxDB response: %w", err)
	}
	return result, nil
}

func (influx *InfluxDB) latestQuery() string {
	return fmt.Sprintf(`from(bucket: "%s")
  |> range(start: -24h)
  |> filter(fn: (r) => r._measurement == "service_check")
  |> pivot(rowKey: ["_time", "service_url"], columnKey: ["_field"], valueColumn: "_value")
  |> sort(columns: ["_time"], desc: true)
  |> limit(n: 1)`, escapeFluxString(influx.bucket))
}

func parseLatestResult(reader io.Reader) (metric.Result, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return metric.Result{}, err
	}
	lines := strings.Split(string(content), "\n")
	dataLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			dataLines = append(dataLines, line)
		}
	}
	if len(dataLines) < 2 {
		return metric.Result{}, ErrNotFound
	}

	records, err := csv.NewReader(strings.NewReader(strings.Join(dataLines, "\n"))).ReadAll()
	if err != nil {
		return metric.Result{}, err
	}
	if len(records) < 2 {
		return metric.Result{}, ErrNotFound
	}
	values := make(map[string]string, len(records[0]))
	for index, column := range records[0] {
		if index < len(records[1]) {
			values[column] = records[1][index]
		}
	}

	checkedAt, err := time.Parse(time.RFC3339Nano, values["_time"])
	if err != nil {
		return metric.Result{}, fmt.Errorf("parse check time: %w", err)
	}
	available, err := strconv.ParseBool(values["available"])
	if err != nil {
		return metric.Result{}, fmt.Errorf("parse availability: %w", err)
	}
	statusCode, err := strconv.Atoi(values["status_code"])
	if err != nil {
		return metric.Result{}, fmt.Errorf("parse status code: %w", err)
	}
	responseTime, err := strconv.ParseInt(values["response_time_ms"], 10, 64)
	if err != nil {
		return metric.Result{}, fmt.Errorf("parse response time: %w", err)
	}

	return metric.Result{
		ServiceURL: values["service_url"], CheckedAt: checkedAt, Available: available,
		StatusCode: statusCode, ResponseTimeMS: responseTime, Error: values["error"],
	}, nil
}

func escapeFluxString(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value)
}
