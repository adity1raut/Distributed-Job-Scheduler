package handler

import (
	"net/http"
	"strconv"

	"github.com/adity1raut/job-scheduler/internal/apperr"
	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func uuidParam(r *http.Request, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		return uuid.Nil, apperr.BadRequest("invalid " + key)
	}
	return id, nil
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, middleware.RequestIDFromContext(r.Context()), err)
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
