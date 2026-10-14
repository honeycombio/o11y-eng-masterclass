package otlpexport

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type receiver struct {
	collectortrace.UnimplementedTraceServiceServer
	spans atomic.Int64
}

func (r *receiver) Export(_ context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			r.spans.Add(int64(len(ss.Spans)))
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func TestNewDeliversForEveryEndpointForm(t *testing.T) {
	tests := []struct {
		name   string
		format func(addr string) string
	}{
		{"bare host:port", func(addr string) string { return addr }},
		{"http URL", func(addr string) string { return "http://" + addr }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			rcv := &receiver{}
			srv := grpc.NewServer()
			collectortrace.RegisterTraceServiceServer(srv, rcv)
			go srv.Serve(lis)
			t.Cleanup(srv.Stop)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			exporter, err := New(ctx, tt.format(lis.Addr().String()))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
			_, span := tp.Tracer("test").Start(ctx, "probe")
			span.End()
			if err := tp.Shutdown(ctx); err != nil {
				t.Fatalf("flushing spans: %v", err)
			}

			if got := rcv.spans.Load(); got != 1 {
				t.Errorf("receiver got %d spans, want 1", got)
			}
		})
	}
}

func TestNewDefaultsToLocalCollector(t *testing.T) {
	exporter, err := New(context.Background(), "")
	if err != nil {
		t.Fatalf("New with empty endpoint: %v", err)
	}
	if err := exporter.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
