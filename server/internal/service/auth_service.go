package service

import (
	"context"
	"time"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/authtoken"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	orgs      *repository.OrganizationRepository
	users     *repository.UserRepository
	policies  *repository.RetryPolicyRepository
	jwtSecret string
	jwtTTL    time.Duration
}

func NewAuthService(orgs *repository.OrganizationRepository, users *repository.UserRepository, policies *repository.RetryPolicyRepository, jwtSecret string, jwtTTL time.Duration) *AuthService {
	return &AuthService{orgs: orgs, users: users, policies: policies, jwtSecret: jwtSecret, jwtTTL: jwtTTL}
}

type AuthResult struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// Register creates a new organization owned by the registering user. Job
// schedulers are naturally multi-tenant (one org per team), so registration
// bootstraps the org rather than requiring an invite flow.
func (s *AuthService) Register(ctx context.Context, orgName, email, password string) (*AuthResult, error) {
	if len(password) < 8 {
		return nil, apperr.BadRequest("password must be at least 8 characters")
	}

	org, err := s.orgs.Create(ctx, orgName)
	if err != nil {
		return nil, apperr.Internal("failed to create organization")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperr.Internal("failed to hash password")
	}

	user, err := s.users.Create(ctx, org.ID, email, string(hash), models.RoleOwner)
	if err != nil {
		if err == repository.ErrConflict {
			return nil, apperr.Conflict("an account with this email already exists")
		}
		return nil, apperr.Internal("failed to create user")
	}

	// Seed a default retry policy so the org can create queues immediately.
	if _, err := s.policies.EnsureDefault(ctx, org.ID); err != nil {
		return nil, apperr.Internal("failed to seed default retry policy")
	}

	token, err := authtoken.Issue(s.jwtSecret, user.ID, user.OrgID, string(user.Role), s.jwtTTL)
	if err != nil {
		return nil, apperr.Internal("failed to issue token")
	}
	return &AuthResult{Token: token, User: user}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, apperr.Unauthorized("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperr.Unauthorized("invalid email or password")
	}

	token, err := authtoken.Issue(s.jwtSecret, user.ID, user.OrgID, string(user.Role), s.jwtTTL)
	if err != nil {
		return nil, apperr.Internal("failed to issue token")
	}
	return &AuthResult{Token: token, User: user}, nil
}
