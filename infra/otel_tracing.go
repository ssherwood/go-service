package infra

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"google.golang.org/grpc/credentials"
	"locationservice/config"
	"log"
	"os"
)

func grpcTracerOptions() []otlptracegrpc.Option {
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.OTELCollectorURL),
		otlptracegrpc.WithCompressor(config.OTELCompressor),
	}

	if config.OTELExporterInsecure == "true" {
		options = append(options, otlptracegrpc.WithInsecure())
	} else {
		options = append(options,
			otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
		)
	}

	return options
}

// InitTracerProvider
// https://opentelemetry.io/docs/languages/go/instrumentation/#traces
func InitTracerProvider(ctx context.Context) (*trace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx, grpcTracerOptions()...)
	if err != nil {
		log.Printf("Unable to initialize OTEL trace exporter: %v\n", err)
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithSampler(trace.AlwaysSample()), // TODO config
		// // set the sampling rate based on the parent span to 60%
		//        trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.6))),
		trace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.TelemetrySDKLanguageGo,
				semconv.ServiceNameKey.String(config.ServiceName),
				semconv.ServiceVersion(config.ServiceVersion),
				semconv.HostNameKey.String(config.Hostname),
				semconv.ProcessPIDKey.Int64(int64(os.Getpid())),
			),
		),
	)

	// set the global tracer provider
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	)

	return tracerProvider, nil
}
