package http

import (
	"encoding/json"
	"net/http"

	"drova/services/trip-service/internal/domain"
	"drova/shared/contracts"

	"go.uber.org/zap"
)

type HttpHandler struct {
	Service domain.TripService
	Log     *zap.SugaredLogger
}

type previewTripRequest struct {
	UserID      string               `json:"userID"`
	Pickup      contracts.Coordinate `json:"pickup"`
	Destination contracts.Coordinate `json:"destination"`
}

func (h *HttpHandler) HandleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	route, err := h.Service.GetRoute(r.Context(), &reqBody.Pickup, &reqBody.Destination)
	if err != nil {
		h.Log.Warnw("get route failed", zap.Error(err))
		http.Error(w, "failed to get route", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(route)
}
