package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"drova/shared/contracts"
	"drova/shared/env"
)

var (
	tripServiceAddr   = env.GetString("TRIP_SERVICE_ADDR", "http://localhost:8083")
	mapboxPublicToken = env.GetString("MAPBOX_PUBLIC_TOKEN", "")
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, contracts.APIResponse{Data: "i am alive"})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"mapboxToken": mapboxPublicToken,
	})
}

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if reqBody.UserID == "" {
		http.Error(w, "user ID is required", http.StatusBadRequest)
		return
	}
	if reqBody.Pickup == "" || reqBody.Destination == "" {
		http.Error(w, "pickup and destination are required", http.StatusBadRequest)
		return
	}

	jsonBody, _ := json.Marshal(reqBody)
	resp, err := http.Post(tripServiceAddr+"/preview", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		http.Error(w, "failed to reach trip service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
