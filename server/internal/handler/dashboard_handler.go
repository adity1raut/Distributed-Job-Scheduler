package handler

import "net/http"

type DashboardHandler struct {
	// TODO: repositories/services needed for aggregate stats
}

func (h *DashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	// TODO: implement queue depth, throughput, failure rate, worker status
}
