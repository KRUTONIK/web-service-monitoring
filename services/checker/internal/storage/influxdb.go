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

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

const measurementName = "service_check"

var ErrNotFound = errors.New("check result not found")

type InfluxDB struct {
	client  *http.Client
	baseURL string
	org     string
	bucket  string
	token   string
}

func NewInfluxDB(client *http.Client, baseURL string, org string, bucket string, token string) *InfluxDB {
	return &InfluxDB{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		org:     org,
		bucket:  bucket,
		token:   token,
	}
}

func (influx *InfluxDB) Write(ctx context.Context, result check.Result) error {
	endpoint, err := url.Parse(influx.baseURL + "/api/v2/write")
	if err != nil {
		return fmt.Errorf("parse InfluxDB URL: %w", err)
	}

	query := endpoint.Query()
	query.Set("org", influx.org)
	query.Set("bucket", influx.bucket)
	query.Set("precision", "ns")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint.String(),
		bytes.NewBufferString(lineProtocol(result)),
	)
	if err != nil {
		return fmt.Errorf("create InfluxDB request: %w", err)
	}
	request.Header.Set("Authorization", "Token "+influx.token)
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")

	response, err := influx.client.Do(request)
	if err != nil {
		return fmt.Errorf("write result to InfluxDB: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("InfluxDB returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func (influx *InfluxDB) Latest(ctx context.Context) (check.Result, error) {
	endpoint, err := url.Parse(influx.baseURL + "/api/v2/query")
	if err != nil {
		return check.Result{}, fmt.Errorf("parse InfluxDB URL: %w", err)
	}

	query := endpoint.Query()
	query.Set("org", influx.org)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint.String(),
		bytes.NewBufferString(influx.latestQuery()),
	)
	if err != nil {
		return check.Result{}, fmt.Errorf("create InfluxDB query request: %w", err)
	}
	request.Header.Set("Authorization", "Token "+influx.token)
	request.Header.Set("Accept", "application/csv")
	request.Header.Set("Content-Type", "application/vnd.flux")

	response, err := influx.client.Do(request)
	if err != nil {
		return check.Result{}, fmt.Errorf("query InfluxDB: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return check.Result{}, fmt.Errorf("InfluxDB returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	result, err := parseLatestResult(response.Body)
	if err != nil {
		return check.Result{}, fmt.Errorf("parse InfluxDB response: %w", err)
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

func parseLatestResult(reader io.Reader) (check.Result, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return check.Result{}, err
	}

	lines := strings.Split(string(content), "\n")
	dataLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			dataLines = append(dataLines, line)
		}
	}
	if len(dataLines) < 2 {
		return check.Result{}, ErrNotFound
	}

	records, err := csv.NewReader(strings.NewReader(strings.Join(dataLines, "\n"))).ReadAll()
	if err != nil {
		return check.Result{}, err
	}
	if len(records) < 2 {
		return check.Result{}, ErrNotFound
	}

	values := make(map[string]string, len(records[0]))
	for index, column := range records[0] {
		if index < len(records[1]) {
			values[column] = records[1][index]
		}
	}

	checkedAt, err := time.Parse(time.RFC3339Nano, values["_time"])
	if err != nil {
		return check.Result{}, fmt.Errorf("parse check time: %w", err)
	}
	available, err := strconv.ParseBool(values["available"])
	if err != nil {
		return check.Result{}, fmt.Errorf("parse availability: %w", err)
	}
	statusCode, err := strconv.Atoi(values["status_code"])
	if err != nil {
		return check.Result{}, fmt.Errorf("parse status code: %w", err)
	}
	responseTime, err := strconv.ParseInt(values["response_time_ms"], 10, 64)
	if err != nil {
		return check.Result{}, fmt.Errorf("parse response time: %w", err)
	}

	return check.Result{
		ServiceURL:     values["service_url"],
		CheckedAt:      checkedAt,
		Available:      available,
		StatusCode:     statusCode,
		ResponseTimeMS: responseTime,
		Error:          values["error"],
	}, nil
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

	return fmt.Sprintf(
		"%s,service_url=%s %s %d",
		measurementName,
		escapeTag(result.ServiceURL),
		strings.Join(fields, ","),
		result.CheckedAt.UnixNano(),
	)
}

func escapeTag(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		",", "\\,",
		" ", "\\ ",
		"=", "\\=",
	)
	return replacer.Replace(value)
}

func escapeFieldString(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
	)
	return replacer.Replace(value)
}

func escapeFluxString(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
	)
	return replacer.Replace(value)
}
