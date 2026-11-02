package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/auth"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/monitoring"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/user"
	"github.com/alexedwards/scs/v2"
)

const testOrigin = "http://localhost:5173"

type stubCheckReader struct {
	result           monitoring.Result
	err              error
	requestedService string
}

func (stub *stubCheckReader) Latest(_ context.Context, serviceID string) (monitoring.Result, error) {
	stub.requestedService = serviceID
	return stub.result, stub.err
}

type stubAuthentication struct {
	registerUser user.User
	registerErr  error
	loginUser    user.User
	loginErr     error
	users        map[string]user.User
}

type stubServiceRepository struct {
	services  []serviceconfig.Service
	service   serviceconfig.Service
	err       error
	created   serviceconfig.Service
	updated   serviceconfig.Service
	deleted   serviceconfig.Service
	ownerID   string
	serviceID string
}

func (stub *stubServiceRepository) ListByOwner(_ context.Context, ownerID string) ([]serviceconfig.Service, error) {
	stub.ownerID = ownerID
	return stub.services, stub.err
}
func (stub *stubServiceRepository) ByIDAndOwner(_ context.Context, ownerID, serviceID string) (serviceconfig.Service, error) {
	stub.ownerID, stub.serviceID = ownerID, serviceID
	return stub.service, stub.err
}
func (stub *stubServiceRepository) Create(_ context.Context, ownerID, _, _ string, _ bool, _, _ int) (serviceconfig.Service, error) {
	stub.ownerID = ownerID
	return stub.created, stub.err
}
func (stub *stubServiceRepository) Update(_ context.Context, ownerID, serviceID, _, _ string, _ bool, _, _ int) (serviceconfig.Service, error) {
	stub.ownerID, stub.serviceID = ownerID, serviceID
	return stub.updated, stub.err
}
func (stub *stubServiceRepository) Delete(_ context.Context, ownerID, serviceID string) (serviceconfig.Service, error) {
	stub.ownerID, stub.serviceID = ownerID, serviceID
	return stub.deleted, stub.err
}

type stubConfigurationPublisher struct {
	update serviceconfig.Update
	err    error
}

func (stub *stubConfigurationPublisher) Publish(_ context.Context, update serviceconfig.Update) error {
	stub.update = update
	return stub.err
}

func (stub *stubAuthentication) Register(_ context.Context, _, _ string) (user.User, error) {
	return stub.registerUser, stub.registerErr
}
func (stub *stubAuthentication) Login(_ context.Context, _, _ string) (user.User, error) {
	return stub.loginUser, stub.loginErr
}
func (stub *stubAuthentication) Current(_ context.Context, id string) (user.User, error) {
	found, ok := stub.users[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return found, nil
}

func newTestHandler(reader checkReader, authentication authentication) (http.Handler, *scs.SessionManager, *stubServiceRepository, *stubConfigurationPublisher) {
	sessions := scs.New()
	services := &stubServiceRepository{}
	publisher := &stubConfigurationPublisher{}
	return sessions.LoadAndSave(New(reader, authentication, sessions, services, publisher, testOrigin)), sessions, services, publisher
}

func authenticatedCookie(t *testing.T, sessions *scs.SessionManager, userID string) *http.Cookie {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	sessions.LoadAndSave(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		sessions.Put(request.Context(), "user_id", userID)
	})).ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected session cookie, got %d", len(cookies))
	}
	return cookies[0]
}

func TestHealth(t *testing.T) {
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestLatestCheckRequiresAuthentication(t *testing.T) {
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/checks/latest", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestLatestCheck(t *testing.T) {
	checkedAt := time.Date(2026, time.September, 25, 10, 30, 0, 0, time.UTC)
	reader := &stubCheckReader{result: monitoring.Result{ServiceID: "service-1", ServiceURL: "https://service.test", CheckedAt: checkedAt, Available: true, StatusCode: http.StatusOK, ResponseTimeMS: 42}}
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1", Email: "test@example.com"}}}
	handler, sessions, services, _ := newTestHandler(reader, authentication)
	services.service = serviceconfig.Service{ID: "service-1"}
	request := httptest.NewRequest(http.MethodGet, "/api/checks/latest?service_id=service-1", nil)
	request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"service_url":"https://service.test"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
	if reader.requestedService != "service-1" {
		t.Fatalf("unexpected requested service: %q", reader.requestedService)
	}
}

func TestLatestCheckErrors(t *testing.T) {
	for name, readerError := range map[string]error{"not found": monitoring.ErrNotFound, "storage failure": errors.New("storage unavailable")} {
		t.Run(name, func(t *testing.T) {
			authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
			handler, sessions, services, _ := newTestHandler(&stubCheckReader{err: readerError}, authentication)
			services.service = serviceconfig.Service{ID: "service-1"}
			request := httptest.NewRequest(http.MethodGet, "/api/checks/latest?service_id=service-1", nil)
			request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			want := http.StatusBadGateway
			if errors.Is(readerError, monitoring.ErrNotFound) {
				want = http.StatusNotFound
			}
			if response.Code != want {
				t.Fatalf("unexpected status: %d", response.Code)
			}
		})
	}
}

func TestRegisterCreatesSession(t *testing.T) {
	authentication := &stubAuthentication{registerUser: user.User{ID: "user-1", Email: "test@example.com"}}
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, authentication)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"test@example.com","password":"correct-horse"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Origin", testOrigin)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d: %s", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 1 {
		t.Fatal("session cookie was not set")
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentialed CORS header was not set")
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{loginErr: auth.ErrInvalidCredentials})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"test@example.com","password":"incorrect"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestMutatingRequestRejectsForeignOrigin(t *testing.T) {
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://foreign.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestServiceSettingsValidation(t *testing.T) {
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
	handler, sessions, _, _ := newTestHandler(&stubCheckReader{}, authentication)
	for name, body := range map[string]string{
		"unsupported interval":    `{"name":"Service","url":"https://service.test","enabled":true,"interval_seconds":15,"timeout_seconds":5}`,
		"timeout equals interval": `{"name":"Service","url":"https://service.test","enabled":true,"interval_seconds":10,"timeout_seconds":10}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/services", strings.NewReader(body))
			request.Header.Set("Origin", testOrigin)
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("unexpected status: %d", response.Code)
			}
		})
	}
}

func TestCORSPreflightForServiceMutations(t *testing.T) {
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
	for name, path := range map[string]string{
		"create": "/api/services",
		"update": "/api/services/service-1",
		"delete": "/api/services/service-1",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, path, nil)
			request.Header.Set("Origin", testOrigin)
			request.Header.Set("Access-Control-Request-Method", map[string]string{
				"create": http.MethodPost,
				"update": http.MethodPut,
				"delete": http.MethodDelete,
			}[name])
			request.Header.Set("Access-Control-Request-Headers", "Content-Type")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("unexpected status: %d", response.Code)
			}
			if response.Header().Get("Access-Control-Allow-Origin") != testOrigin {
				t.Fatalf("unexpected allowed origin: %q", response.Header().Get("Access-Control-Allow-Origin"))
			}
			if response.Header().Get("Access-Control-Allow-Methods") != "GET, POST, PUT, DELETE, OPTIONS" {
				t.Fatalf("unexpected allowed methods: %q", response.Header().Get("Access-Control-Allow-Methods"))
			}
			if response.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
				t.Fatalf("unexpected allowed headers: %q", response.Header().Get("Access-Control-Allow-Headers"))
			}
			if response.Header().Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatal("credentialed CORS header was not set")
			}
		})
	}
}

func TestCORSPreflightRejectsForeignOrigin(t *testing.T) {
	handler, _, _, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
	request := httptest.NewRequest(http.MethodOptions, "/api/services/service-1", nil)
	request.Header.Set("Origin", "https://foreign.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign origin was allowed: %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCreateServicePublishesConfiguration(t *testing.T) {
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
	handler, sessions, services, publisher := newTestHandler(&stubCheckReader{}, authentication)
	services.created = serviceconfig.Service{ID: "service-1", Name: "Основной сайт", URL: "https://example.test", Enabled: true, Version: 1}
	request := httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewBufferString(`{"name":"Основной сайт","url":"https://example.test","enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", testOrigin)
	request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d: %s", response.Code, response.Body.String())
	}
	if services.ownerID != "user-1" {
		t.Fatalf("unexpected service owner: %q", services.ownerID)
	}
	if publisher.update.Event != "service.updated" || publisher.update.Service.ID != "service-1" {
		t.Fatalf("unexpected configuration update: %+v", publisher.update)
	}
}

func TestDeleteServiceDisablesConfiguration(t *testing.T) {
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
	handler, sessions, services, publisher := newTestHandler(&stubCheckReader{}, authentication)
	services.deleted = serviceconfig.Service{ID: "service-1", Enabled: false, Version: 2}
	request := httptest.NewRequest(http.MethodDelete, "/api/services/service-1", nil)
	request.Header.Set("Origin", testOrigin)
	request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if services.serviceID != "service-1" || publisher.update.Service.Enabled {
		t.Fatalf("unexpected deletion update: %+v", publisher.update)
	}
}

func TestSuccessfulServiceOperationsIgnorePublicationFailure(t *testing.T) {
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
	for name, expectedStatus := range map[string]int{"create": http.StatusCreated, "update": http.StatusOK, "delete": http.StatusNoContent} {
		t.Run(name, func(t *testing.T) {
			handler, sessions, services, publisher := newTestHandler(&stubCheckReader{}, authentication)
			publisher.err = errors.New("RabbitMQ unavailable")
			service := serviceconfig.Service{ID: "service-1", Name: "Основной сайт", URL: "https://example.test", Enabled: name != "delete", Version: 2}
			services.created, services.updated, services.deleted = service, service, service
			method, path, body := http.MethodPost, "/api/services", `{"name":"Основной сайт","url":"https://example.test","enabled":true}`
			if name == "update" {
				method, path = http.MethodPut, "/api/services/service-1"
			}
			if name == "delete" {
				method, path, body = http.MethodDelete, "/api/services/service-1", ""
			}
			request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
			request.Header.Set("Origin", testOrigin)
			if name != "delete" {
				request.Header.Set("Content-Type", "application/json")
			}
			request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != expectedStatus {
				t.Fatalf("unexpected status: %d: %s", response.Code, response.Body.String())
			}
			if publisher.update.Service.ID != "service-1" {
				t.Fatalf("configuration was not published: %+v", publisher.update)
			}
		})
	}
}

func TestServiceSaveFailureDoesNotPublishConfiguration(t *testing.T) {
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
	handler, sessions, services, publisher := newTestHandler(&stubCheckReader{}, authentication)
	services.err = errors.New("database unavailable")
	request := httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewBufferString(`{"name":"Основной сайт","url":"https://example.test","enabled":true}`))
	request.Header.Set("Origin", testOrigin)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || publisher.update.Event != "" {
		t.Fatalf("unexpected response=%d update=%+v", response.Code, publisher.update)
	}
}

func TestDeleteServiceRejectsForeignOwnerAndRepeatedDeletion(t *testing.T) {
	authentication := &stubAuthentication{users: map[string]user.User{"user-1": {ID: "user-1"}}}
	for name, repositoryError := range map[string]error{
		"чужой сервис":       serviceconfig.ErrNotFound,
		"повторное удаление": serviceconfig.ErrNotFound,
	} {
		t.Run(name, func(t *testing.T) {
			handler, sessions, services, publisher := newTestHandler(&stubCheckReader{}, authentication)
			services.err = repositoryError
			request := httptest.NewRequest(http.MethodDelete, "/api/services/service-2", nil)
			request.Header.Set("Origin", testOrigin)
			request.AddCookie(authenticatedCookie(t, sessions, "user-1"))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("unexpected status: %d", response.Code)
			}
			if publisher.update.Event != "" {
				t.Fatalf("unexpected update: %+v", publisher.update)
			}
		})
	}
}
