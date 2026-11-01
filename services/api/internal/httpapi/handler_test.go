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

func newTestHandler(reader checkReader, authentication authentication) (http.Handler, *scs.SessionManager) {
	sessions := scs.New()
	return sessions.LoadAndSave(New(reader, authentication, sessions, testOrigin)), sessions
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
	handler, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
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
	handler, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
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
	handler, sessions := newTestHandler(reader, authentication)
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
			handler, sessions := newTestHandler(&stubCheckReader{err: readerError}, authentication)
			request := httptest.NewRequest(http.MethodGet, "/api/checks/latest", nil)
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
	handler, _ := newTestHandler(&stubCheckReader{}, authentication)
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
	handler, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{loginErr: auth.ErrInvalidCredentials})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"test@example.com","password":"incorrect"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestMutatingRequestRejectsForeignOrigin(t *testing.T) {
	handler, _ := newTestHandler(&stubCheckReader{}, &stubAuthentication{})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://foreign.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}
