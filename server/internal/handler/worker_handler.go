package handler

import (
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/service"
)

type WorkerHandler struct {
	workers *service.WorkerService
}

func NewWorkerHandler(workers *service.WorkerService) *WorkerHandler {
	return &WorkerHandler{workers: workers}
}

func (h *WorkerHandler) List(w http.ResponseWriter, r *http.Request) {
	fleet, err := h.workers.Fleet(r.Context())
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
	beats, err := h.workers.Heartbeats(r.Context(), id, queryInt(r, "limit", 50))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, beats)
}
