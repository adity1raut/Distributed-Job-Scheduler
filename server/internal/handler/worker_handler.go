package handler

import (
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/adity1raut/job-scheduler/internal/service"
)

type WorkerHandler struct {
	workers *service.WorkerService
}

func NewWorkerHandler(workers *service.WorkerService) *WorkerHandler {
	return &WorkerHandler{workers: workers}
}

func (h *WorkerHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	fleet, err := h.workers.Fleet(r.Context(), orgID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, fleet)
}

func (h *WorkerHandler) Heartbeats(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "workerID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	beats, err := h.workers.Heartbeats(r.Context(), id, orgID, queryInt(r, "limit", 50))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, beats)
}
