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

	// Retry topics — failed messages are re-attempted with increasing delays.
	// retry-1: 1 min, retry-2: 5 min, retry-3: 30 min, then DLQ.
	TopicRetry1 = "drova.retry.1"
	TopicRetry2 = "drova.retry.2"
	TopicRetry3 = "drova.retry.3"
)
