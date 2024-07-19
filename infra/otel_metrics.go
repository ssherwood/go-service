package infra

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"google.golang.org/grpc/credentials"
	"locationservice/config"
	"os"
	"time"
)

func grpcMetricOptions() []otlpmetricgrpc.Option {
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(config.CollectorURL),
		otlpmetricgrpc.WithCompressor("gzip"),
	}

	if config.Insecure == "true" {
		options = append(options, otlpmetricgrpc.WithInsecure())
	} else {
		options = append(options, otlpmetricgrpc.WithTLSCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		))
	}

	return options
}

// InitializeMetricProvider
// https://opentelemetry.io/docs/languages/go/instrumentation/#metrics
func InitializeMetricProvider(ctx context.Context) (*metric.MeterProvider, error) {

	exporter, err := otlpmetricgrpc.New(ctx, grpcMetricOptions()...)
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(exporter, metric.WithInterval(10*time.Second)), // emit every 10s
		),
		metric.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.TelemetrySDKLanguageGo,
				semconv.ServiceName(config.ServiceName),
				semconv.ServiceVersion(config.ServiceVersion),
				semconv.HostNameKey.String(config.Hostname),
				semconv.ProcessPIDKey.Int64(int64(os.Getpid())),
			),
		),
	)

	// set the global meter provider
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}
