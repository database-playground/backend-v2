package auth

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("dbplay.auth")
