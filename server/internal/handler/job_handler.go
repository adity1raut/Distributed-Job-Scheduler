package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/adity1raut/job-scheduler/internal/repository"
	"github.com/adity1raut/job-scheduler/internal/service"
	"github.com/google/uuid"
)

type JobHandler struct {
	jobs *service.JobService
}

func NewJobHandler(jobs *service.JobService) *JobHandler {
	return &JobHandler{jobs: jobs}
}

type submitJobRequest struct {
	Type           models.JobType  `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey *string         `json:"idempotency_key"`
	Priority       int             `json:"priority"`
	DelayMS        int             `json:"delay_ms"`
	RunAt          *time.Time      `json:"run_at"`
	RetryPolicyID  *uuid.UUID      `json:"retry_policy_id"`
	BatchCount     int             `json:"batch_count"`
}

func (h *JobHandler) Submit(w http.ResponseWriter, r *http.Request) {
	queueID, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var req submitJobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	jobs, err := h.jobs.Submit(r.Context(), orgID, queueID, service.SubmitJobInput{
		Type:           req.Type,
		Payload:        req.Payload,
		IdempotencyKey: req.IdempotencyKey,
		Priority:       req.Priority,
		DelayMS:        req.DelayMS,
		RunAt:          req.RunAt,
		RetryPolicyID:  req.RetryPolicyID,
		BatchCount:     req.BatchCount,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, jobs)
}

func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	queueID, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}

	filter := repository.JobFilter{
		QueueID: queueID,
		Limit:   queryInt(r, "limit", httpx.DefaultPageLimit),
	}
	if status := r.URL.Query().Get("status"); status != "" {
		s := models.JobStatus(status)
		filter.Status = &s
	}
	if jobType := r.URL.Query().Get("type"); jobType != "" {
		t := models.JobType(jobType)
		filter.Type = &t
	}
	if cursor, ok := httpx.DecodeCursor(r.URL.Query().Get("cursor")); ok {
		filter.Cursor = cursor
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	page, err := h.jobs.List(r.Context(), orgID, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "jobID")
	if err != nil {
		writeErr(w, r, err)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	job, err := h.jobs.Get(r.Context(), orgID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	executions, err := h.jobs.Executions(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, struct {
		*models.Job
		Executions []models.JobExecution `json:"executions"`
	}{Job: job, Executions: executions})
}

func (h *JobHandler) Logs(w http.ResponseWriter, r *http.Request) {
	executionID, err := uuidParam(r, "executionID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	logs, err := h.jobs.Logs(r.Context(), orgID, executionID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, logs)
}

func (h *JobHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "jobID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	job, err := h.jobs.Retry(r.Context(), orgID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, job)
}
