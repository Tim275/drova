package http

import (
	"encoding/json"
	"log"
	"net/http"

	"drova/services/trip-service/internal/domain"
)

type HttpHandler struct {
	Service domain.TripService
}

type previewTripRequest struct {
	UserID      string `json:"userID"`
	Pickup      string `json:"pickup"`
	Destination string `json:"destination"`
}

func (h *HttpHandler) HandleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	route, err := h.Service.GetRoute(r.Context(), reqBody.Pickup, reqBody.Destination)
	if err != nil {
		log.Println(err)
		http.Error(w, "failed to get route", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(route)
}
