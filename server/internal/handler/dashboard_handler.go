package handler

import (
	"net/http"

	"github.com/adity1raut/job-scheduler/internal/httpx"
	"github.com/adity1raut/job-scheduler/internal/middleware"
	"github.com/adity1raut/job-scheduler/internal/service"
)

type DashboardHandler struct {
	dashboard *service.DashboardService
}

func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	overview, err := h.dashboard.Overview(r.Context(), orgID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, overview)
}
