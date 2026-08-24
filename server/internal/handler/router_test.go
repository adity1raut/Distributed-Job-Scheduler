package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/testutil"
	"golang.org/x/crypto/bcrypt"
)

var testEmailCounter int64

// uniqueSuffix keeps fixture emails unique both within one test run
// (email has a UNIQUE constraint, and multiple tests here register their
// own org) and across repeated runs against the same persistent test
// database (a bare in-process counter would restart at 1 every run).
func uniqueSuffix() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&testEmailCounter, 1))
}

// These exercise the actual router — real handlers, real middleware chain
// (auth, RBAC, rate limiting, CORS) — rather than calling a service method
// directly. They require a real Postgres (TEST_DATABASE_URL) and a
// reachable Redis, same as the other integration tests in this package.

type authResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		OrgID string `json:"org_id"`
		Role  string `json:"role"`
	} `json:"user"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doJSON(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func registerOrg(t *testing.T, router http.Handler, orgName, email string) authResponse {
	t.Helper()
	rec := doJSON(t, router, http.MethodPost, "/api/auth/register", "", map[string]string{
		"organization_name": orgName,
		"email":              email,
		"password":           "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var res authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	return res
}

func TestRouter_Healthz_NeedsNoAuth(t *testing.T) {
	pool := testutil.RequireDB(t)
	router := testutil.NewTestRouter(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouter_RegisterThenLogin_FullFlow(t *testing.T) {
	pool := testutil.RequireDB(t)
	router := testutil.NewTestRouter(t, pool)

	email := fmt.Sprintf("owner-%s@example.com", uniqueSuffix())
	reg := registerOrg(t, router, "Acme", email)
	if reg.Token == "" {
		t.Fatal("register did not return a token")
	}
	if reg.User.Role != "owner" {
		t.Fatalf("expected registering user to be owner, got %q", reg.User.Role)
	}

	rec := doJSON(t, router, http.MethodPost, "/api/auth/login", "", map[string]string{
		"email":    email,
		"password": "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRouter_ProtectedRoute_RejectsMissingBearerToken(t *testing.T) {
	pool := testutil.RequireDB(t)
	router := testutil.NewTestRouter(t, pool)

	rec := doJSON(t, router, http.MethodGet, "/api/projects", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	var errBody errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errBody.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("expected code UNAUTHORIZED, got %q", errBody.Error.Code)
	}
}

func TestRouter_SubmitJob_InvalidTypeReturnsStructuredBadRequest(t *testing.T) {
	pool := testutil.RequireDB(t)
	router := testutil.NewTestRouter(t, pool)

	email := fmt.Sprintf("badtype-%s@example.com", uniqueSuffix())
	reg := registerOrg(t, router, "BadType Inc", email)

	rec := doJSON(t, router, http.MethodPost, "/api/projects", reg.Token, map[string]string{"name": "proj"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &project)

	rec = doJSON(t, router, http.MethodPost, "/api/projects/"+project.ID+"/queues", reg.Token, map[string]any{
		"name": "q1", "priority": 0, "concurrency_limit": 5,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create queue: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var queue struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &queue)

	rec = doJSON(t, router, http.MethodPost, "/api/queues/"+queue.ID+"/jobs", reg.Token, map[string]any{
		"type": "not-a-real-type",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("submit job with bad type: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var errBody errorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Error.Code != "BAD_REQUEST" {
		t.Fatalf("expected code BAD_REQUEST, got %q", errBody.Error.Code)
	}
}

// TestRouter_DeleteProject_RoleGate is the RBAC regression test: a member
// account (created directly, since there's no invite endpoint yet — see
// design-decisions.md) must be forbidden from deleting a project, while the
// owner who created it must still be allowed to.
func TestRouter_DeleteProject_RoleGate(t *testing.T) {
	pool := testutil.RequireDB(t)
	router := testutil.NewTestRouter(t, pool)

	ownerEmail := fmt.Sprintf("owner-%s@example.com", uniqueSuffix())
	owner := registerOrg(t, router, "RoleGate Inc", ownerEmail)

	rec := doJSON(t, router, http.MethodPost, "/api/projects", owner.Token, map[string]string{"name": "gated-proj"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &project)

	userRepo := repository.NewUserRepository(pool)
	memberEmail := fmt.Sprintf("member-%s@example.com", uniqueSuffix())
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	orgUUID := testutil.MustParseUUID(t, owner.User.OrgID)
	if _, err := userRepo.Create(t.Context(), orgUUID, memberEmail, string(hash), models.RoleMember); err != nil {
		t.Fatalf("create member user: %v", err)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/auth/login", "", map[string]string{
		"email":    memberEmail,
		"password": "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("member login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var memberAuth authResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &memberAuth)
	if memberAuth.User.Role != "member" {
		t.Fatalf("expected role member, got %q", memberAuth.User.Role)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/projects/"+project.ID, memberAuth.Token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member delete: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var errBody errorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Error.Code != "FORBIDDEN" {
		t.Fatalf("expected code FORBIDDEN, got %q", errBody.Error.Code)
	}

	// The owner who actually created it must still be able to delete it.
	rec = doJSON(t, router, http.MethodDelete, "/api/projects/"+project.ID, owner.Token, nil)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("owner delete: expected 200/204, got %d: %s", rec.Code, rec.Body.String())
	}
}
