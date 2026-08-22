package handler

import (
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/service"
)

type DLQHandler struct {
	dlq *service.DLQService
}

func NewDLQHandler(dlq *service.DLQService) *DLQHandler {
	return &DLQHandler{dlq: dlq}
}

func (h *DLQHandler) List(w http.ResponseWriter, r *http.Request) {
	queueID, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	entries, err := h.dlq.List(r.Context(), queueID, queryInt(r, "limit", 50))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entries)
}

func (h *DLQHandler) Replay(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "entryID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	job, err := h.dlq.Replay(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, job)
}
