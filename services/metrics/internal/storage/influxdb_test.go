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
