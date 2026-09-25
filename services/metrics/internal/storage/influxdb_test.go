package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`#datatype,string,long,dateTime:RFC3339,string,boolean,long,long,string
#group,false,false,false,true,false,false,false,false
#default,_result,,,,,,,
,result,table,_time,service_url,available,status_code,response_time_ms,error
,,0,2026-09-25T10:30:00.123Z,https://service.test,true,204,27,
`))
	}))
	defer server.Close()

	result, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Latest(context.Background())
	if err != nil {
		t.Fatalf("get latest result: %v", err)
	}
	if result.ServiceURL != "https://service.test" || !result.Available || result.StatusCode != 204 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLatestNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "#datatype,string\n")
	}))
	defer server.Close()

	_, err := NewInfluxDB(server.Client(), server.URL, "org", "bucket", "token").Latest(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
