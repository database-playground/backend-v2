package authservice

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("dbplay.httpapi.auth")
