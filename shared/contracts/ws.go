package contracts

const (
	DriverCmdRegister    = "driver.cmd.register"
	DriverCmdTripRequest = "driver.cmd.trip_request"
	DriverCmdTripAccept  = "driver.cmd.trip_accept"
	DriverCmdTripDecline = "driver.cmd.trip_decline"
	DriverCmdLocation    = "driver.cmd.location"
	DriverCmdArrived     = "driver.cmd.arrived"
	DriverCmdTripStart   = "driver.cmd.trip_start"
	DriverCmdTripEnd    = "driver.cmd.trip_end"
	DriverCmdTripCancel = "driver.cmd.trip_cancel"

	TripCmdCancel = "trip.cmd.cancel"

	TripEventDriverAssigned      = "trip.event.driver_assigned"
	TripEventNoDriversFound      = "trip.event.no_drivers_found"
	TripEventDriverNotInterested = "trip.event.driver_not_interested"
	TripEventCancelled          = "trip.event.cancelled"
	TripEventCancelledByDriver  = "trip.event.cancelled_by_driver"
	TripEventDriverArrived       = "trip.event.driver_arrived"
	TripEventInProgress          = "trip.event.in_progress"
	TripEventCompleted           = "trip.event.completed"
)

type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}
