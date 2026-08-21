package service

import "github.com/adity1raut/job-scheduler/internal/repository"

// AuthService handles signup, login, and JWT issuance/verification.
type AuthService struct {
	users     repository.UserRepository
	jwtSecret string
}

func NewAuthService(users repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret}
}

func (s *AuthService) Register(email, password string) error {
	// TODO: hash password, create user
	return nil
}

func (s *AuthService) Login(email, password string) (string, error) {
	// TODO: verify password, issue JWT
	return "", nil
}
