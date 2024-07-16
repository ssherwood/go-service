package infra

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc/credentials"
	"locationservice/common"
	"log"
)

var (
	serviceName  = common.GetEnv("SERVICE_NAME", "location-service")
	collectorURL = common.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	insecure     = common.GetEnv("INSECURE_MODE", "true")
)

func grpcOptions() []otlptracegrpc.Option {
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(collectorURL),
		otlptracegrpc.WithCompressor("gzip"),
	}

	if insecure == "true" {
		options = append(options, otlptracegrpc.WithInsecure())
	} else {
		options = append(options, otlptracegrpc.WithTLSCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		))
	}

	return options
}

func InitTracerProvider(ctx context.Context) (*trace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx, grpcOptions()...)
	if err != nil {
		log.Fatalf("error: %s", err.Error())
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithSampler(trace.AlwaysSample()), // TODO config
		// // set the sampling rate based on the parent span to 60%
		//        trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.6))),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.TelemetrySDKLanguageGo,
		)),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tracerProvider, nil
}