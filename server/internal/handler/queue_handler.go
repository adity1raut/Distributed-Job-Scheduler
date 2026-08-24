package handler

import (
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/adity1raut/job-scheduler/internal/service"
	"github.com/google/uuid"
)

type QueueHandler struct {
	queues *service.QueueService
}

func NewQueueHandler(queues *service.QueueService) *QueueHandler {
	return &QueueHandler{queues: queues}
}

type createQueueRequest struct {
	Name             string     `json:"name"`
	Priority         int        `json:"priority"`
	ConcurrencyLimit int        `json:"concurrency_limit"`
	RetryPolicyID    *uuid.UUID `json:"retry_policy_id"`
}

func (h *QueueHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuidParam(r, "projectID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var req createQueueRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	queue, err := h.queues.Create(r.Context(), orgID, projectID, service.CreateQueueInput{
		Name:             req.Name,
		Priority:         req.Priority,
		ConcurrencyLimit: req.ConcurrencyLimit,
		RetryPolicyID:    req.RetryPolicyID,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, queue)
}

func (h *QueueHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuidParam(r, "projectID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())

	queues, err := h.queues.List(r.Context(), orgID, projectID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, queues)
}

func (h *QueueHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	queue, err := h.queues.Get(r.Context(), orgID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, queue)
}

type updateQueueRequest struct {
	Priority         *int       `json:"priority"`
	ConcurrencyLimit *int       `json:"concurrency_limit"`
	RetryPolicyID    *uuid.UUID `json:"retry_policy_id"`
}

func (h *QueueHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var req updateQueueRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	queue, err := h.queues.UpdateConfig(r.Context(), orgID, id, service.UpdateQueueInput{
		Priority:         req.Priority,
		ConcurrencyLimit: req.ConcurrencyLimit,
		RetryPolicyID:    req.RetryPolicyID,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, queue)
}

func (h *QueueHandler) Pause(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, true)
}

func (h *QueueHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, false)
}

func (h *QueueHandler) setPaused(w http.ResponseWriter, r *http.Request, paused bool) {
	id, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	if err := h.queues.SetPaused(r.Context(), orgID, id, paused); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"is_paused": paused})
}

func (h *QueueHandler) Stats(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	stats, err := h.queues.Stats(r.Context(), orgID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

func (h *QueueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "queueID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())
	if err := h.queues.Delete(r.Context(), orgID, id); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}
