package check

import (
 "context"
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestRunHTTPStatusBoundaries(t *testing.T) {
 tests := []struct {
  status int
  available bool
 }{
  {200, true},
  {299, true},
  {300, false},
  {404, false},
  {500, false},
 }
 for _, tc := range tests {
  t.Run(http.StatusText(tc.status), func(t *testing.T) {
   server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(tc.status)
   }))
   defer server.Close()
   result := New(server.Client()).Run(context.Background(), "service-1", server.URL)
   if result.Available != tc.available || result.StatusCode != tc.status {
    t.Fatalf("status %d: got available=%v, code=%d", tc.status, result.Available, result.StatusCode)
   }
  })
 }
}
