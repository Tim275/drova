package schema

import _ "embed"

//go:embed avsc/trip_created.avsc
var tripCreatedAvsc string

//go:embed avsc/trip_status_event.avsc
var tripStatusEventAvsc string

//go:embed avsc/driver_trip_request.avsc
var driverTripRequestAvsc string

//go:embed avsc/payment_status_update.avsc
var paymentStatusUpdateAvsc string

var (
	SchemaTripCreated         = MustParseSchema(tripCreatedAvsc)
	SchemaTripStatus          = MustParseSchema(tripStatusEventAvsc)
	SchemaDriverTripRequest   = MustParseSchema(driverTripRequestAvsc)
	SchemaPaymentStatusUpdate = MustParseSchema(paymentStatusUpdateAvsc)
)
