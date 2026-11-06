package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/user"
	"golang.org/x/crypto/bcrypt"
)

type fakeUsers struct {
	created   user.User
	createErr error
	byEmail   user.User
	emailErr  error
	byID      user.User
	idErr     error
}

func (fake *fakeUsers) Create(_ context.Context, value user.User) (user.User, error) {
	fake.created = value
	if fake.createErr != nil {
		return user.User{}, fake.createErr
	}
	value.ID = "user-1"
	return value, nil
}
func (fake *fakeUsers) ByEmail(context.Context, string) (user.User, error) {
	return fake.byEmail, fake.emailErr
}
func (fake *fakeUsers) ByID(context.Context, string) (user.User, error) { return fake.byID, fake.idErr }

func TestRegister(t *testing.T) {
	t.Run("normalizes email and hashes password", func(t *testing.T) {
		users := &fakeUsers{}
		created, err := (&Service{users: users}).Register(context.Background(), " Owner@Example.Test ", "long password")
		if err != nil || created.ID != "user-1" {
			t.Fatalf("register = %+v, %v", created, err)
		}
		if users.created.Email != "owner@example.test" || users.created.PasswordHash == "long password" {
			t.Fatalf("created user = %+v", users.created)
		}
		if bcrypt.CompareHashAndPassword([]byte(users.created.PasswordHash), []byte("long password")) != nil {
			t.Fatal("stored password hash does not match password")
		}
	})
	for _, test := range []struct{ name, email, password string }{
		{"invalid email", "not-an-email", "long password"},
		{"short password", "owner@example.test", "short"},
		{"long password", "owner@example.test", strings.Repeat("p", 73)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&Service{users: &fakeUsers{}}).Register(context.Background(), test.email, test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("register error = %v", err)
			}
		})
	}
	t.Run("maps duplicate email", func(t *testing.T) {
		_, err := (&Service{users: &fakeUsers{createErr: user.ErrEmailTaken}}).Register(context.Background(), "owner@example.test", "long password")
		if !errors.Is(err, ErrEmailTaken) {
			t.Fatalf("register error = %v", err)
		}
	})
}

func TestLoginAndCurrent(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("long password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUsers{byEmail: user.User{ID: "user-1", PasswordHash: string(hash)}, byID: user.User{ID: "user-1"}}
	service := &Service{users: users}
	if found, err := service.Login(context.Background(), "owner@example.test", "long password"); err != nil || found.ID != "user-1" {
		t.Fatalf("login = %+v, %v", found, err)
	}
	if _, err := service.Login(context.Background(), "owner@example.test", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	users.emailErr = user.ErrNotFound
	if _, err := service.Login(context.Background(), "missing@example.test", "long password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("missing user error = %v", err)
	}
	if found, err := service.Current(context.Background(), "user-1"); err != nil || found.ID != "user-1" {
		t.Fatalf("current = %+v, %v", found, err)
	}
	users.idErr = errors.New("database unavailable")
	if _, err := service.Current(context.Background(), "user-1"); err == nil {
		t.Fatal("expected current user repository error")
	}
}
