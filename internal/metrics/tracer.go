package metrics

import (
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("dbplay.metrics")
