package admin

import (
	_ "embed"
	"net/http"
)

//go:embed dashboard.html
var adminDashboardHTML []byte

func (h *AdminHandler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(adminDashboardHTML)
}
