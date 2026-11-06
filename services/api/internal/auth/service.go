package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/user"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrEmailTaken = errors.New("email already registered")

type userRepository interface {
	Create(context.Context, user.User) (user.User, error)
	ByEmail(context.Context, string) (user.User, error)
	ByID(context.Context, string) (user.User, error)
}

type Service struct{ users userRepository }

func New(users *user.Repository) *Service { return &Service{users: users} }

func (service *Service) Register(ctx context.Context, email, password string) (user.User, error) {
	email = user.NormalizeEmail(email)
	if _, err := mail.ParseAddress(email); err != nil || email == "" {
		return user.User{}, ErrInvalidCredentials
	}
	if len(password) < 10 || len(password) > 72 {
		return user.User{}, ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return user.User{}, fmt.Errorf("hash password: %w", err)
	}
	created, err := service.users.Create(ctx, user.User{Email: email, PasswordHash: string(hash)})
	if errors.Is(err, user.ErrEmailTaken) {
		return user.User{}, ErrEmailTaken
	}
	if err != nil {
		return user.User{}, fmt.Errorf("create user: %w", err)
	}
	return created, nil
}

func (service *Service) Login(ctx context.Context, email, password string) (user.User, error) {
	found, err := service.users.ByEmail(ctx, email)
	if err != nil {
		return user.User{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(found.PasswordHash), []byte(password)) != nil {
		return user.User{}, ErrInvalidCredentials
	}
	return found, nil
}

func (service *Service) Current(ctx context.Context, id string) (user.User, error) {
	return service.users.ByID(ctx, id)
}
