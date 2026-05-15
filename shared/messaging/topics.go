package messaging

const (
	TopicTripCreated         = "trip.event.created"
	TopicDriverTripRequest   = "driver.cmd.trip_request"
	TopicDriverTripResponse  = "driver.cmd.trip_response"
	TopicTripDriverAssigned  = "trip.event.driver_assigned"
	TopicTripNoDriversFound  = "trip.event.no_drivers_found"
	TopicDriverNotInterested = "driver.event.not_interested"
	TopicDeadLetterQueue     = "dead.letter.queue"

	TopicDriverLocation = "driver.cmd.location"

	TopicTripCancelled     = "trip.event.cancelled"
	TopicTripDriverArrived = "trip.event.driver_arrived"
	TopicTripInProgress    = "trip.event.in_progress"
	TopicTripCompleted     = "trip.event.completed"

	TopicPaymentCreateSession  = "payment.cmd.create_session"
	TopicPaymentSessionCreated = "payment.event.session_created"
	TopicPaymentSuccess        = "payment.event.success"
)
