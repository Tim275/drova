package types

import pb "drova/shared/proto/trip"

type MapboxRouteResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"routes"`
}

func (o *MapboxRouteResponse) ToProto() *pb.Route {
	route := o.Routes[0]
	coords := route.Geometry.Coordinates
	pbCoords := make([]*pb.Coordinate, len(coords))
	for i, c := range coords {
		pbCoords[i] = &pb.Coordinate{
			Longitude: c[0],
			Latitude:  c[1],
		}
	}
	return &pb.Route{
		Geometry: []*pb.Geometry{{Coordinates: pbCoords}},
		Distance: route.Distance,
		Duration: route.Duration,
	}
}

type PricingConfig struct {
	PricePerUnitOfDistance float64
	PricingPerMinute       float64
}

func DefaultPricingConfig() *PricingConfig {
	return &PricingConfig{
		PricePerUnitOfDistance: 150, // cents per km (= 1.50€/km)
		PricingPerMinute:       30,  // cents per min (= 0.30€/min)
	}
}
