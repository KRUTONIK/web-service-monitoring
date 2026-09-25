package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/monitoring"
)

type checkReader interface {
	Latest(context.Context) (monitoring.Result, error)
}

type handler struct {
	checks checkReader
}

func New(checks checkReader) http.Handler {
	handler := &handler{checks: checks}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /api/checks/latest", handler.latestCheck)
	return allowCORS(mux)
}

func (handler *handler) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *handler) latestCheck(writer http.ResponseWriter, request *http.Request) {
	result, err := handler.checks.Latest(request.Context())
	if errors.Is(err, monitoring.ErrNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "check result not found"})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"error": "failed to read monitoring data"})
		return
	}

	writeJSON(writer, http.StatusOK, result)
}

func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
