package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/auth"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/monitoring"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
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

type serviceRepository interface {
	ListByOwner(context.Context, string) ([]serviceconfig.Service, error)
	ByIDAndOwner(context.Context, string, string) (serviceconfig.Service, error)
	Create(context.Context, string, string, string, bool) (serviceconfig.Service, error)
	Update(context.Context, string, string, string, string, bool) (serviceconfig.Service, error)
	Delete(context.Context, string, string) (serviceconfig.Service, error)
}

type configurationPublisher interface {
	Publish(context.Context, serviceconfig.Update) error
}

type handler struct {
	checks    checkReader
	auth      authentication
	sessions  *scs.SessionManager
	services  serviceRepository
	publisher configurationPublisher
	origin    string
}

func New(checks checkReader, authService authentication, sessions *scs.SessionManager, services serviceRepository, publisher configurationPublisher, origin string) http.Handler {
	handler := &handler{checks: checks, auth: authService, sessions: sessions, services: services, publisher: publisher, origin: origin}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("POST /api/auth/register", handler.register)
	mux.HandleFunc("POST /api/auth/login", handler.login)
	mux.HandleFunc("POST /api/auth/logout", handler.logout)
	mux.HandleFunc("GET /api/auth/me", handler.me)
	mux.HandleFunc("GET /api/services", handler.listServices)
	mux.HandleFunc("POST /api/services", handler.createService)
	mux.HandleFunc("PUT /api/services/{id}", handler.updateService)
	mux.HandleFunc("DELETE /api/services/{id}", handler.deleteService)
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

type serviceInput struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
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

func (handler *handler) listServices(writer http.ResponseWriter, request *http.Request) {
	currentUser, ok := handler.currentUser(writer, request)
	if !ok {
		return
	}
	services, err := handler.services.ListByOwner(request.Context(), currentUser.ID)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось получить сервисы"})
		return
	}

	writeJSON(writer, http.StatusOK, services)
}

func (handler *handler) createService(writer http.ResponseWriter, request *http.Request) {
	currentUser, ok := handler.currentUser(writer, request)
	if !ok {
		return
	}
	input, ok := handler.decodeServiceInput(writer, request)
	if !ok {
		return
	}
	service, err := handler.services.Create(request.Context(), currentUser.ID, input.Name, input.URL, input.Enabled)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось создать сервис"})
		return
	}
	handler.publishConfiguration(request, "create", service)

	writeJSON(writer, http.StatusCreated, service)
}

func (handler *handler) updateService(writer http.ResponseWriter, request *http.Request) {
	currentUser, ok := handler.currentUser(writer, request)
	if !ok {
		return
	}
	input, ok := handler.decodeServiceInput(writer, request)
	if !ok {
		return
	}
	service, err := handler.services.Update(request.Context(), currentUser.ID, request.PathValue("id"), input.Name, input.URL, input.Enabled)
	if errors.Is(err, serviceconfig.ErrNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "Сервис не найден"})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось изменить сервис"})
		return
	}
	handler.publishConfiguration(request, "update", service)

	writeJSON(writer, http.StatusOK, service)
}

func (handler *handler) deleteService(writer http.ResponseWriter, request *http.Request) {
	if !handler.requireOrigin(writer, request) {
		return
	}
	currentUser, ok := handler.currentUser(writer, request)
	if !ok {
		return
	}
	service, err := handler.services.Delete(request.Context(), currentUser.ID, request.PathValue("id"))
	if errors.Is(err, serviceconfig.ErrNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "Сервис не найден"})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось удалить сервис"})
		return
	}
	handler.publishConfiguration(request, "delete", service)

	writer.WriteHeader(http.StatusNoContent)
}

func (handler *handler) latestCheck(writer http.ResponseWriter, request *http.Request) {
	currentUser, ok := handler.currentUser(writer, request)
	if !ok {
		return
	}
	serviceID := request.URL.Query().Get("service_id")
	if serviceID == "" {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "Не указан сервис"})
		return
	}
	if _, err := handler.services.ByIDAndOwner(request.Context(), currentUser.ID, serviceID); errors.Is(err, serviceconfig.ErrNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "Сервис не найден"})
		return
	} else if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "Не удалось проверить доступ к сервису"})
		return
	}
	result, err := handler.checks.Latest(request.Context(), serviceID)
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

func (handler *handler) decodeServiceInput(writer http.ResponseWriter, request *http.Request) (serviceInput, bool) {
	if !handler.requireOrigin(writer, request) {
		return serviceInput{}, false
	}
	contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"error": "Требуется JSON"})
		return serviceInput{}, false
	}
	var input serviceInput
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "Некорректный JSON"})
		return serviceInput{}, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	if len(input.Name) == 0 || len(input.Name) > 100 {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "Укажите название сервиса длиной до 100 символов"})
		return serviceInput{}, false
	}
	if len(input.URL) > 2048 || !isHTTPURL(input.URL) {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "Укажите корректный HTTP(S) URL"})
		return serviceInput{}, false
	}

	return input, true
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func (handler *handler) publishConfiguration(request *http.Request, operation string, service serviceconfig.Service) {
	if err := handler.publisher.Publish(request.Context(), serviceconfig.Update{Event: "service.updated", Service: service}); err != nil {
		log.Printf("publish configuration operation=%s service_id=%s: %v", operation, service.ID, err)
	}
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
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
