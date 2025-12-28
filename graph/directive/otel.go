package directive

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("dbplay.graphql.directive")
