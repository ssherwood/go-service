package shared

import (
	"context"
	"github.com/ssherwood/locationservice/internal/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
	"log/slog"
	"os"
	"time"
)

func grpcLogOptions() []otlploggrpc.Option {
	options := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(config.OTELCollectorURL),
		otlploggrpc.WithCompressor(config.OTELCompressor),
	}

	if config.OTELExporterInsecure {
		options = append(options, otlploggrpc.WithInsecure())
	} else {
		options = append(options, otlploggrpc.WithTLSCredentials(
			credentials.NewClientTLSFromCert(nil, ""),
		))
	}

	return options
}

func InitializeLoggingProvider(ctx context.Context) (*sdklog.LoggerProvider, error) {
	//stdoutExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	//if err != nil {
	//	slog.Warn("Failed to initialize OTEL log stdoutExporter", config.ErrAttr(err))
	//}

	grpcExporter, err := otlploggrpc.New(ctx, grpcLogOptions()...)
	if err != nil {
		slog.Error("Unable to initialize OTEL log grpcExporter", config.ErrAttr(err))
		return nil, err
	}

	//stdoutExporter, err := stdoutlogs.NewExporter()
	//if err != nil {
	//	slog.Error("Unable to initialize OTEL log grpcExporter", config.ErrAttr(err))
	//	return nil, err
	//}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(grpcExporter)),
		//log.WithProcessor(log.NewSimpleProcessor(stdoutExporter)),
		sdklog.WithResource(
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

	global.SetLoggerProvider(provider)

	return provider, nil
}

type OTLPLogHandler struct {
	consoleHandler slog.Handler
	exporter       *otlploggrpc.Exporter
}

func NewOTLPLogHandler(consoleHandler slog.Handler, exporter *otlploggrpc.Exporter) *OTLPLogHandler {
	return &OTLPLogHandler{consoleHandler: consoleHandler, exporter: exporter}
}

func (h *OTLPLogHandler) Handle(ctx context.Context, rec slog.Record) error {
	// Log to console
	if err := h.consoleHandler.Handle(ctx, rec); err != nil {
		return err
	}

	//spanContext := trace.SpanFromContext(ctx).SpanContext()
	//var traceID *trace.TraceID = nil
	//var spanID *trace.SpanID = nil
	//var traceFlags *trace.TraceFlags = nil
	//if spanContext.IsValid() {
	//	tid := spanContext.TraceID()
	//	sid := spanContext.SpanID()
	//	tf := spanContext.TraceFlags()
	//	traceID = &tid
	//	spanID = &sid
	//	traceFlags = &tf
	//}
	//levelString := rec.Level.String()
	//severity := SeverityNumber(int(rec.Level.Level()) + 9)
	//
	//var attributes []attribute.KeyValue

	//for _, attr := range o.attrs {
	//	attributes = append(attributes, otelAttribute(attr)...)
	//}
	//
	//rec.Attrs(func(attr slog.Attr) bool {
	//	attributes = append(attributes, otelAttribute(withGroupPrefix(o.groupPrefix, attr))...)
	//	return true
	//})
	//
	//lrc := LogRecordConfig{
	//	Timestamp:            &rec.Time,
	//	ObservedTimestamp:    rec.Time,
	//	TraceId:              traceID,
	//	SpanId:               spanID,
	//	TraceFlags:           traceFlags,
	//	SeverityText:         &levelString,
	//	SeverityNumber:       &severity,
	//	Body:                 &rec.Message,
	//	Resource:             nil,
	//	InstrumentationScope: &instrumentationScope,
	//	Attributes:           &attributes,
	//}
	//
	//r := NewLogRecord(lrc)
	//o.logger.Emit(r)

	return nil
}

type LogRecordConfig struct {
	Timestamp            *time.Time
	ObservedTimestamp    time.Time
	TraceId              *trace.TraceID
	SpanId               *trace.SpanID
	TraceFlags           *trace.TraceFlags
	SeverityText         *string
	SeverityNumber       *SeverityNumber
	Body                 *string
	Resource             *resource.Resource
	InstrumentationScope *instrumentation.Scope
	Attributes           *[]attribute.KeyValue
}

func NewLogRecord(config LogRecordConfig) LogRecord {
	return LogRecord{
		timestamp:            config.Timestamp,
		observedTimestamp:    config.ObservedTimestamp,
		traceId:              config.TraceId,
		spanId:               config.SpanId,
		traceFlags:           config.TraceFlags,
		severityText:         config.SeverityText,
		severityNumber:       config.SeverityNumber,
		body:                 config.Body,
		resource:             config.Resource,
		instrumentationScope: config.InstrumentationScope,
		attributes:           config.Attributes,
	}
}

type LogRecord struct {
	timestamp            *time.Time
	observedTimestamp    time.Time
	traceId              *trace.TraceID
	spanId               *trace.SpanID
	traceFlags           *trace.TraceFlags
	severityText         *string
	severityNumber       *SeverityNumber
	body                 *string
	resource             *resource.Resource
	instrumentationScope *instrumentation.Scope
	attributes           *[]attribute.KeyValue
}

// SeverityNumber Possible values for LogRecord.SeverityNumber.
type SeverityNumber int32

//func (h *OTLPSlogHandler) Handle(ctx context.Context, rec slog.Record) error {
//	// Log to console
//	if err := h.consoleHandler.Handle(ctx, rec); err != nil {
//		return err
//	}
//
//	// Create a span for logging
//	_, span := h.tracer.Start(ctx, "log")
//	defer span.End()
//
//	span.SetAttributes(attribute.String("log.message", rec.Message))
//	span.SetAttributes(attribute.String("log.level", rec.Level.String()))
//	// Iterate over attributes and set them
//	rec.Attrs(func(attr slog.Attr) bool {
//		span.SetAttributes(attribute.String(attr.Key, attr.Value.String()))
//		return true
//	})
//
//	return nil
//}
