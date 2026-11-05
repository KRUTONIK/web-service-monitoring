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

func TestHistoryQueryAggregatesAllJobsIntoTimeWindows(t *testing.T) {
	query := NewInfluxDB(nil, "", "", "bucket", "").historyQuery("service-1", time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), 1000, 0, time.Hour)
	group := `|> group(columns: ["service_id", "service_url", "_field"])`
	aggregate := `|> aggregateWindow(every: 1h0m0s, fn: last, createEmpty: false)`
	if !strings.Contains(query, group) || !strings.Contains(query, aggregate) || strings.Index(query, group) > strings.Index(query, aggregate) || strings.Contains(query, `rowKey: ["_time", "job_id"`) {
		t.Fatalf("unexpected aggregate query: %s", query)
	}
}

func TestSummaryQueryHasNoLimitForLargePeriods(t *testing.T) {
	query := NewInfluxDB(nil, "", "", "bucket", "").summaryQuery("service-1", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), `|> count(column: "_value")`)
	if strings.Contains(query, "limit(") || !strings.Contains(query, `range(start: 2026-11-01T00:00:00Z)`) || !strings.Contains(query, `r.service_id == "service-1"`) {
		t.Fatalf("summary query must cover the full period without a page limit: %s", query)
	}
}

func TestSummaryResponseTimeIsConvertedToFloatForP95(t *testing.T) {
	query := NewInfluxDB(nil, "", "", "bucket", "").summaryQuery("service-1", time.Now().Add(-time.Hour), `|> map(fn: (r) => ({r with response_time_ms: float(v: r.response_time_ms)}))
  |> quantile(column: "response_time_ms", q: 0.95, method: "exact_selector")`)
	if !strings.Contains(query, `float(v: r.response_time_ms)`) || !strings.Contains(query, `quantile(column: "response_time_ms", q: 0.95, method: "exact_selector")`) {
		t.Fatalf("P95 query must convert response_time_ms to float: %s", query)
	}
}

func TestSummaryParsesAggregateResponses(t *testing.T) {
	values := []string{"2", "1", "0", "0"}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		if requestCount > len(values) {
			t.Fatal("unexpected aggregate query")
		}
		_, _ = io.WriteString(writer, "#datatype,string,long\n#group,false,false\n#default,_result,,\n,result,table,_value\n,,0,"+values[requestCount-1]+"\n")
	}))
	defer server.Close()
	summary, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Summary(context.Background(), "service-1", time.Now().Add(-time.Hour))
	if err != nil || summary.TotalChecks != 2 || summary.SuccessfulChecks != 1 || !summary.HasResponseMetrics || summary.AverageResponseMS != 0 || summary.P95ResponseMS != 0 || requestCount != 4 {
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
