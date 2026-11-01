package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/auth"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/monitoring"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/user"
	"github.com/alexedwards/scs/v2"
)

type checkReader interface {
	Latest(context.Context, string) (monitoring.Result, error)
}

type authentication interface {
	Register(context.Context, string, string) (user.User, error)
	Login(context.Context, string, string) (user.User, error)
	Current(context.Context, string) (user.User, error)
}

type handler struct {
	checks   checkReader
	auth     authentication
	sessions *scs.SessionManager
	origin   string
}

func New(checks checkReader, authService authentication, sessions *scs.SessionManager, origin string) http.Handler {
	handler := &handler{checks: checks, auth: authService, sessions: sessions, origin: origin}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("POST /api/auth/register", handler.register)
	mux.HandleFunc("POST /api/auth/login", handler.login)
	mux.HandleFunc("POST /api/auth/logout", handler.logout)
	mux.HandleFunc("GET /api/auth/me", handler.me)
	mux.HandleFunc("GET /api/checks/latest", handler.latestCheck)
	return handler.allowCORS(mux)
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type currentUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (handler *handler) register(writer http.ResponseWriter, request *http.Request) {
	credentials, ok := handler.decodeCredentials(writer, request)
	if !ok {
		return
	}
	created, err := handler.auth.Register(request.Context(), credentials.Email, credentials.Password)
	if errors.Is(err, auth.ErrEmailTaken) {
		writeJSON(writer, http.StatusConflict, map[string]string{"error": "Электронная почта уже зарегистрирована"})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "Некорректные данные регистрации"})
		return
	}
	handler.sessions.Put(request.Context(), "user_id", created.ID)
	writeJSON(writer, http.StatusCreated, currentUser{ID: created.ID, Email: created.Email})
}

func (handler *handler) login(writer http.ResponseWriter, request *http.Request) {
	credentials, ok := handler.decodeCredentials(writer, request)
	if !ok {
		return
	}
	found, err := handler.auth.Login(request.Context(), credentials.Email, credentials.Password)
	if err != nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "Неверная электронная почта или пароль"})
		return
	}
	if err := handler.sessions.RenewToken(request.Context()); err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось обновить сессию"})
		return
	}
	handler.sessions.Put(request.Context(), "user_id", found.ID)
	writeJSON(writer, http.StatusOK, currentUser{ID: found.ID, Email: found.Email})
}

func (handler *handler) logout(writer http.ResponseWriter, request *http.Request) {
	if !handler.requireOrigin(writer, request) {
		return
	}
	if err := handler.sessions.Destroy(request.Context()); err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось завершить сессию"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *handler) me(writer http.ResponseWriter, request *http.Request) {
	found, ok := handler.currentUser(writer, request)
	if !ok {
		return
	}
	writeJSON(writer, http.StatusOK, currentUser{ID: found.ID, Email: found.Email})
}

func (handler *handler) latestCheck(writer http.ResponseWriter, request *http.Request) {
	if _, ok := handler.currentUser(writer, request); !ok {
		return
	}
	result, err := handler.checks.Latest(request.Context(), request.URL.Query().Get("service_id"))
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

func (handler *handler) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *handler) decodeCredentials(writer http.ResponseWriter, request *http.Request) (credentials, bool) {
	if !handler.requireOrigin(writer, request) {
		return credentials{}, false
	}
	contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"error": "Требуется JSON"})
		return credentials{}, false
	}
	var value credentials
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "Некорректный JSON"})
		return credentials{}, false
	}
	return value, true
}

func (handler *handler) currentUser(writer http.ResponseWriter, request *http.Request) (user.User, bool) {
	if handler.sessions == nil || handler.auth == nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Аутентификация не настроена"})
		return user.User{}, false
	}
	id := handler.sessions.GetString(request.Context(), "user_id")
	if id == "" {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "Требуется аутентификация"})
		return user.User{}, false
	}
	found, err := handler.auth.Current(request.Context(), id)
	if err != nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "Требуется аутентификация"})
		return user.User{}, false
	}
	return found, true
}

func (handler *handler) requireOrigin(writer http.ResponseWriter, request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" || origin == handler.origin {
		return true
	}
	writeJSON(writer, http.StatusForbidden, map[string]string{"error": "Недопустимый источник запроса"})
	return false
}

func (handler *handler) allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin != "" && origin == handler.origin {
			writer.Header().Set("Access-Control-Allow-Origin", handler.origin)
			writer.Header().Set("Access-Control-Allow-Credentials", "true")
			writer.Header().Set("Vary", "Origin")
		}
		if request.Method == http.MethodOptions {
			if origin != handler.origin {
				writeJSON(writer, http.StatusForbidden, map[string]string{"error": "Недопустимый источник запроса"})
				return
			}
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
