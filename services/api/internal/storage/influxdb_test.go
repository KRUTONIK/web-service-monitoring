package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/query" {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("org") != "test-org" {
			t.Errorf("unexpected organization: %s", request.URL.Query().Get("org"))
		}
		if request.Header.Get("Authorization") != "Token test-token" {
			t.Errorf("unexpected authorization header: %s", request.Header.Get("Authorization"))
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read query: %v", err)
		}
		query := string(body)
		for _, expected := range []string{`from(bucket: "test-bucket")`, `r._measurement == "service_check"`, "|> pivot", "|> limit(n: 1)"} {
			if !strings.Contains(query, expected) {
				t.Errorf("query does not contain %q: %s", expected, query)
			}
		}

		writer.Header().Set("Content-Type", "text/csv")
		_, _ = writer.Write([]byte(`#datatype,string,long,dateTime:RFC3339,string,boolean,long,long,string
#group,false,false,false,true,false,false,false,false
#default,_result,,,,,,,
,result,table,_time,service_url,available,status_code,response_time_ms,error
,,0,2026-09-25T10:30:00.123Z,https://service.test,true,204,27,
`))
	}))
	defer server.Close()

	influx := NewInfluxDB(server.Client(), server.URL, "test-org", "test-bucket", "test-token")
	result, err := influx.Latest(context.Background())

	if err != nil {
		t.Fatalf("get latest result: %v", err)
	}
	if result.ServiceURL != "https://service.test" {
		t.Fatalf("unexpected service URL: %s", result.ServiceURL)
	}
	if !result.Available || result.StatusCode != http.StatusNoContent || result.ResponseTimeMS != 27 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.CheckedAt.Format("2006-01-02T15:04:05.999Z07:00") != "2026-09-25T10:30:00.123Z" {
		t.Fatalf("unexpected check time: %s", result.CheckedAt)
	}
}

func TestLatestNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/csv")
		_, _ = writer.Write([]byte("#datatype,string\n"))
	}))
	defer server.Close()

	influx := NewInfluxDB(server.Client(), server.URL, "test-org", "test-bucket", "test-token")
	_, err := influx.Latest(context.Background())

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestLatestReturnsInfluxDBError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("invalid token"))
	}))
	defer server.Close()

	influx := NewInfluxDB(server.Client(), server.URL, "test-org", "test-bucket", "bad-token")
	_, err := influx.Latest(context.Background())

	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("unexpected error: %v", err)
	}
}
