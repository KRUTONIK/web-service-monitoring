package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestLatest(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read query: %v", err)
		}
		receivedQuery = string(body)
		_, _ = writer.Write([]byte(`#datatype,string,long,dateTime:RFC3339,string,boolean,long,long,string
#group,false,false,false,true,false,false,false,false
#default,_result,,,,,,,
,result,table,_time,service_id,service_url,available,status_code,response_time_ms,error
,,0,2026-09-25T10:30:00.123Z,service-1,https://service.test,true,204,27,
`))
	}))
	defer server.Close()

	result, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Latest(context.Background(), "service-1")
	if err != nil {
		t.Fatalf("get latest result: %v", err)
	}
	if result.ServiceID != "service-1" || result.ServiceURL != "https://service.test" || !result.Available || result.StatusCode != 204 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(receivedQuery, `r.service_id == "service-1"`) {
		t.Fatalf("missing service ID filter in query: %s", receivedQuery)
	}
	if strings.Index(receivedQuery, "|> group()") > strings.Index(receivedQuery, `|> sort(columns: ["_time"], desc: true)`) || strings.Index(receivedQuery, `|> sort(columns: ["_time"], desc: true)`) > strings.Index(receivedQuery, "|> limit(n: 1)") {
		t.Fatalf("latest query must globally sort records after pivot: %s", receivedQuery)
	}
}

func TestLatestParsesNewestResultAcrossDifferentJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`#datatype,string,long,dateTime:RFC3339,string,string,boolean,long,long,string
#group,false,false,false,true,true,false,false,false,false
#default,_result,,,,,,,,
,result,table,_time,service_id,job_id,service_url,available,status_code,response_time_ms,error
,,0,2026-11-02T10:00:00Z,service-1,job-new,https://new.test,true,200,21,
,,1,2026-11-02T09:00:00Z,service-1,job-old,https://old.test,false,503,99,old failure
`))
	}))
	defer server.Close()

	result, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Latest(context.Background(), "service-1")
	if err != nil {
		t.Fatalf("get latest result: %v", err)
	}
	if result.ServiceURL != "https://new.test" || result.CheckedAt.Format(time.RFC3339) != "2026-11-02T10:00:00Z" {
		t.Fatalf("expected newest result from a distinct job, got %+v", result)
	}
}

func TestLatestNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "#datatype,string\n")
	}))
	defer server.Close()

	_, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Latest(context.Background(), "service-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestLatestManyParsesSeparateInfluxCSVTables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `#datatype,string,long,dateTime:RFC3339,string,string,boolean,long,long,string
#group,false,false,false,true,true,false,false,false,false
#default,_result,,,,,,,,
,result,table,_time,service_id,service_url,available,status_code,response_time_ms,error
,,0,2026-11-05T10:00:00Z,service-a,https://a.test,true,200,15,
#datatype,string,long,dateTime:RFC3339,string,string,boolean,long,long,string
#group,false,false,false,true,true,false,false,false,false
#default,_result,,,,,,,,
,result,table,_time,service_id,service_url,available,status_code,response_time_ms,error
,,1,2026-11-05T10:01:00Z,service-b,https://b.test,false,503,25,unavailable
`)
	}))
	defer server.Close()

	results, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").LatestMany(context.Background(), []string{"service-a", "service-b", "missing"})
	if err != nil {
		t.Fatalf("get latest results: %v", err)
	}
	if len(results) != 2 || results[0].ServiceID != "service-a" || results[0].ResponseTimeMS != 15 || results[1].ServiceID != "service-b" || results[1].StatusCode != 503 || results[1].Error != "unavailable" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestParseResultsAcceptsInfluxCRLFResponse(t *testing.T) {
	results, err := parseResults(strings.NewReader(",result,table,_time,service_id,service_url,available,status_code,response_time_ms\r\n,,0,2026-11-05T10:00:00Z,service-a,https://a.test,true,200,15\r\n"))
	if err != nil || len(results) != 1 || results[0].StatusCode != 200 || results[0].ResponseTimeMS != 15 {
		t.Fatalf("unexpected results=%+v err=%v", results, err)
	}
}

func TestParseResultsRejectsDataWithoutRequiredColumn(t *testing.T) {
	_, err := parseResults(strings.NewReader(",result,table,_time,service_id,available,status_code,response_time_ms\n,,0,2026-11-05T10:00:00Z,service-a,true,200,15\n"))
	if err == nil || !strings.Contains(err.Error(), "service_url") {
		t.Fatalf("expected missing column error, got %v", err)
	}
}

func TestParseResultsRejectsMalformedCSVAndTimestamp(t *testing.T) {
	for _, test := range []struct {
		name, csv, want string
	}{
		{"malformed CSV", `,result,"unterminated` + "\n", "parse InfluxDB CSV row"},
		{"invalid timestamp", ",result,table,_time,service_id,service_url,available,status_code,response_time_ms\n,,0,not-a-time,service-a,https://a.test,true,200,15\n", "parse check time"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseResults(strings.NewReader(test.csv))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parse error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLatestManyEmptyDoesNotCallInfluxDB(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("InfluxDB must not be queried for an empty service list")
		return nil, nil
	})}
	results, err := NewInfluxDB(client, "http://influx.test", "org", "bucket", "token").LatestMany(context.Background(), nil)
	if err != nil || len(results) != 0 {
		t.Fatalf("results = %+v, error = %v", results, err)
	}
}

func TestHistoryQueryKeepsOneFailedCheckPerTimeWindow(t *testing.T) {
	query := NewInfluxDB(nil, "", "", "bucket", "").historyQuery("service-1", time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), 1000, 0, time.Hour)
	pivot := `|> pivot(rowKey: ["_time", "service_id", "service_url"], columnKey: ["_field"], valueColumn: "_value")`
	window := `|> window(every: 1h0m0s)`
	reduce := `|> reduce(`
	if !strings.Contains(query, pivot) || !strings.Contains(query, window) || !strings.Contains(query, reduce) || strings.Index(query, pivot) > strings.Index(query, window) || strings.Index(query, window) > strings.Index(query, reduce) || strings.Contains(query, "aggregateWindow") || !strings.Contains(query, `accumulator.available and not r.available`) {
		t.Fatalf("unexpected aggregate query: %s", query)
	}
}

func TestHistoryParsesRepresentativeFailureWithZeroResponseTime(t *testing.T) {
	results, err := parseResults(strings.NewReader(`,result,table,_time,service_id,service_url,available,status_code,response_time_ms,error
,,0,2026-11-05T12:01:00Z,service-1,https://service.test,false,503,0,temporary failure
,,1,2026-11-05T12:05:00Z,service-1,https://service.test,true,200,17,
`))
	if err != nil || len(results) != 2 || results[0].Available || results[0].ResponseTimeMS != 0 || results[0].StatusCode != 503 || results[0].Error != "temporary failure" || !results[1].Available {
		t.Fatalf("unexpected representative results=%+v err=%v", results, err)
	}
}

func TestSummaryQueryHasNoLimitForLargePeriods(t *testing.T) {
	query := NewInfluxDB(nil, "", "", "bucket", "").summaryQuery("service-1", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), `|> count(column: "_value")`)
	if strings.Contains(query, "limit(") || !strings.Contains(query, `range(start: 2026-11-01T00:00:00Z)`) || !strings.Contains(query, `r.service_id == "service-1"`) {
		t.Fatalf("summary query must cover the full period without a page limit: %s", query)
	}
}

func TestSummaryResponseTimeIsConvertedToValueForAggregates(t *testing.T) {
	query := NewInfluxDB(nil, "", "", "bucket", "").summaryQuery("service-1", time.Now().Add(-time.Hour), `|> map(fn: (r) => ({r with _value: float(v: r.response_time_ms)}))
  |> quantile(q: 0.95, method: "exact_selector")`)
	if !strings.Contains(query, `_value: float(v: r.response_time_ms)`) || !strings.Contains(query, `quantile(q: 0.95, method: "exact_selector")`) {
		t.Fatalf("P95 query must convert response_time_ms to _value: %s", query)
	}
}

func TestSummaryParsesAggregateResponses(t *testing.T) {
	values := []string{"2", "1", "0", "0"}
	requestCount := 0
	queries := make([]string, 0, len(values))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		if requestCount > len(values) {
			t.Fatal("unexpected aggregate query")
		}
		query, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read aggregate query: %v", err)
		}
		queries = append(queries, string(query))
		_, _ = io.WriteString(writer, "#datatype,string,long\n#group,false,false\n#default,_result,,\n,result,table,_value\n,,0,"+values[requestCount-1]+"\n")
	}))
	defer server.Close()
	summary, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Summary(context.Background(), "service-1", time.Now().Add(-time.Hour))
	if err != nil || summary.TotalChecks != 2 || summary.SuccessfulChecks != 1 || !summary.HasResponseMetrics || summary.AverageResponseMS != 0 || summary.P95ResponseMS != 0 || requestCount != 4 {
		t.Fatalf("unexpected summary=%+v err=%v requests=%d", summary, err, requestCount)
	}
	if !strings.Contains(queries[0], `count(column: "_value")`) || !strings.Contains(queries[1], `count(column: "_value")`) || !strings.Contains(queries[2], `_value: float(v: r.response_time_ms)`) || !strings.Contains(queries[2], `mean()`) || !strings.Contains(queries[3], `_value: float(v: r.response_time_ms)`) || !strings.Contains(queries[3], `quantile(q: 0.95, method: "exact_selector")`) {
		t.Fatalf("unexpected aggregate queries: %v", queries)
	}
}

func TestSummarySkipsResponseMetricsWithoutSuccessfulChecks(t *testing.T) {
	values := []string{"2", "0"}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requestCount >= len(values) {
			t.Fatal("response metrics must not be requested without successful checks")
		}
		_, _ = io.WriteString(writer, "#datatype,string,long\n#group,false,false\n#default,_result,,\n,result,table,_value\n,,0,"+values[requestCount]+"\n")
		requestCount++
	}))
	defer server.Close()

	summary, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Summary(context.Background(), "service-1", time.Now().Add(-time.Hour))
	if err != nil || summary.TotalChecks != 2 || summary.SuccessfulChecks != 0 || summary.HasResponseMetrics || requestCount != 2 {
		t.Fatalf("unexpected summary=%+v err=%v requests=%d", summary, err, requestCount)
	}
}

func TestScalarIncludesLimitedInfluxDBErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(writer, "Flux parsing failed: "+strings.Repeat("x", 1024)+"not-in-error")
	}))
	defer server.Close()

	_, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").scalar(context.Background(), "invalid flux")
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") || !strings.Contains(err.Error(), "Flux parsing failed") || strings.Contains(err.Error(), "not-in-error") {
		t.Fatalf("unexpected error: %v", err)
	}
}
