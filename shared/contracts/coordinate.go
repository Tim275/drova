package contracts

// Coordinate is a lat/long pair shared across services (formerly shared/types).
type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
