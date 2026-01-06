package metrics

import (
	"time"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("dbplay.metrics")

const ScrapeTimeout = 30 * time.Second
