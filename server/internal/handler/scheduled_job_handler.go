package handler

import (
	"encoding/json"
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/service"
)

type ScheduledJobHandler struct {
	scheduledJobs *service.ScheduledJobService
}

func NewScheduledJobHandler(scheduledJobs *service.ScheduledJobService) *ScheduledJobHandler {
	return &ScheduledJobHandler{scheduledJobs: scheduledJobs}
}

type createScheduledJobRequest struct {
	CronExpression  string          `json:"cron_expression"`
	PayloadTemplate json.RawMessage `json:"payload_template"`
}

func (h *ScheduledJobHandler) Create(w http.ResponseWriter, r *http.Request) {
	queueID, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var req createScheduledJobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}

	sj, err := h.scheduledJobs.Create(r.Context(), queueID, req.CronExpression, req.PayloadTemplate)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, sj)
}

func (h *ScheduledJobHandler) List(w http.ResponseWriter, r *http.Request) {
	queueID, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	list, err := h.scheduledJobs.List(r.Context(), queueID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *ScheduledJobHandler) Pause(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *ScheduledJobHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

func (h *ScheduledJobHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := uuidParam(r, "scheduledJobID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := h.scheduledJobs.SetActive(r.Context(), id, active); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"is_active": active})
}
