// Package otlpexport builds the OTLP gRPC trace exporter the seeders share.
//
// OTEL_EXPORTER_OTLP_ENDPOINT is a URL (http://localhost:4317) in the OTel
// spec and for Claude Code's own export, which session 6 points at the same
// shell, but a bare host:port (localhost:4317) is what the seeders documented
// first. Both forms are accepted: gRPC would otherwise read a URL as a
// hostname and fail with "too many colons in address".
package otlpexport

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

const defaultEndpoint = "localhost:4317"

// New returns an exporter for endpoint, which may be a URL or a host:port, and
// defaults to the local Collector when empty. A bare host:port is plaintext,
// since the seeders only ever talk to the local Collector that way.
func New(ctx context.Context, endpoint string) (*otlptrace.Exporter, error) {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if strings.Contains(endpoint, "://") {
		return otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
	}
	return otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
}
