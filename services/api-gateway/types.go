package main

type previewTripRequest struct {
	UserID      string `json:"userID"`
	Pickup      string `json:"pickup"`      // z.B. "Marienplatz, München"
	Destination string `json:"destination"` // z.B. "Flughafen München"
}
