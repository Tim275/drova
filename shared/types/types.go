package types

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type MapboxGeocodeResponse struct {
	Features []struct {
		Center []float64 `json:"center"` // [lng, lat]
	} `json:"features"`
}

type MapboxRouteResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"` // meters
		Duration float64 `json:"duration"` // seconds
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"routes"`
}
