package handler

import (
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/adity1raut/job-scheduler/internal/service"
)

type ProjectHandler struct {
	projects *service.ProjectService
}

func NewProjectHandler(projects *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projects: projects}
}

type createProjectRequest struct {
	Name string `json:"name"`
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeErr(w, r, err)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	project, err := h.projects.Create(r.Context(), orgID, userID, req.Name)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, project)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	projects, err := h.projects.List(r.Context(), orgID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, projects)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "projectID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())

	project, err := h.projects.Get(r.Context(), orgID, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, project)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuidParam(r, "projectID")
	if err != nil {
		writeErr(w, r, err)
		return
	}
	orgID := middleware.OrgIDFromContext(r.Context())

	if err := h.projects.Delete(r.Context(), orgID, id); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}
