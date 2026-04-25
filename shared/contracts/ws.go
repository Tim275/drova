package contracts

const (
	DriverCmdRegister    = "driver.cmd.register"
	DriverCmdTripRequest = "driver.cmd.trip_request"
	DriverCmdTripAccept  = "driver.cmd.trip_accept"
	DriverCmdTripDecline = "driver.cmd.trip_decline"
	DriverCmdLocation    = "driver.cmd.location"

	TripEventDriverAssigned     = "trip.event.driver_assigned"
	TripEventNoDriversFound     = "trip.event.no_drivers_found"
	TripEventDriverNotInterested = "trip.event.driver_not_interested"
)

type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}
