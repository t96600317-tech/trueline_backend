package user

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"trueline-backend/internal/auth"
)

type UserHandler struct {
	service *UserService
}

func NewUserHandler(service *UserService) *UserHandler {
	return &UserHandler{service: service}
}

type UpdateLanguagePayload struct {
	Language string `json:"language"`
}

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *errorBody  `json:"error,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{Success: true, Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{
		Success: false,
		Error:   &errorBody{Code: code, Message: message},
	})
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	u, balanceMicros, err := h.service.GetUserProfile(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	// Convert micros to coins for display
	balanceCoins := float64(balanceMicros) / 1000000.0

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":    u,
		"balance": balanceCoins,
	})
}

func (h *UserHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	_ = h.service.Heartbeat(r.Context(), claims.UserID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "online"})
}

type UpdateProfilePayload struct {
	Name string `json:"name"`
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req UpdateProfilePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	u, err := h.service.UpdateProfile(r.Context(), claims.UserID, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, u)
}

func (h *UserHandler) UpdateLanguage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req UpdateLanguagePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	u, err := h.service.UpdateLanguage(r.Context(), claims.UserID, req.Language)
	if err != nil {
		writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, u)
}

func (h *UserHandler) DiscoverListeners(w http.ResponseWriter, r *http.Request) {
	var currentUserID uuid.UUID
	claims, ok := auth.ClaimsFromContext(r.Context())
	if ok && claims != nil {
		currentUserID = claims.UserID
	}

	langFilter := r.URL.Query().Get("language")
	searchQuery := r.URL.Query().Get("search")

	listeners, err := h.service.ListDiscoverListeners(r.Context(), currentUserID, langFilter, searchQuery)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DISCOVER_FAILED", err.Error())
		return
	}

	// Map RatePerMinMicros to coins for frontend compatibility
	type ListenerResponse struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Title          string   `json:"title"`
		PhotoUrl       string   `json:"photo_url"`
		AudioSampleUrl string   `json:"audio_sample_url"`
		Bio            string   `json:"bio"`
		Languages      []string `json:"languages"`
		RatePerMin     float64  `json:"rate_per_min"`
		RatingAvg      float64  `json:"rating_avg"`
		RatingCount    int      `json:"rating_count"`
		Availability   string   `json:"availability"`
		IsFavourite    bool     `json:"is_favourite"`
	}

	resp := make([]ListenerResponse, len(listeners))
	for i, l := range listeners {
		resp[i] = ListenerResponse{
			ID:             l.ID.String(),
			Name:           l.Name,
			Title:          l.Title,
			PhotoUrl:       l.PhotoURL,
			AudioSampleUrl: l.AudioSampleURL,
			Bio:            l.Bio,
			Languages:      l.Languages,
			RatePerMin:     float64(l.RatePerMinMicros) / 1000000.0,
			RatingAvg:      l.RatingAvg,
			RatingCount:    l.RatingCount,
			Availability:   l.Availability,
			IsFavourite:    l.IsFavourite,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
