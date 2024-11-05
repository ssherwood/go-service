package shared

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ssherwood/ysqlapp/internal/config"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
	"log"
	"runtime/debug"
	"strings"
)

const (
	tracerName          = "github.com/ssherwood/otelpgx"
	sqlOperationUnknown = "UNKNOWN"
)

const (
	// RowsAffectedKey represents the number of rows affected.
	RowsAffectedKey = attribute.Key("pgx.rows_affected")
	// QueryParametersKey represents the query parameters.
	QueryParametersKey = attribute.Key("pgx.query.parameters")
	// BatchSizeKey represents the batch size.
	BatchSizeKey = attribute.Key("pgx.batch.size")
	// PrepareStmtNameKey represents the prepared statement name.
	PrepareStmtNameKey = attribute.Key("pgx.prepare_stmt.name")
	// SQLStateKey represents PostgreSQL error code,
	// see https://www.postgresql.org/docs/current/errcodes-appendix.html.
	SQLStateKey = attribute.Key("pgx.sql_state")
)

type SpanNameFunc func(stmt string) string

type PgxQueryTracer struct {
	tracer              trace.Tracer
	attrs               []attribute.KeyValue
	trimQuerySpanName   bool
	spanNameFunc        SpanNameFunc
	prefixQuerySpanName bool
	logSQLStatement     bool
	includeParams       bool
}

func (t *PgxQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	log.Println(">>> Query Start:", "sql:", data.SQL, "args:", data.Args)

	if !trace.SpanFromContext(ctx).IsRecording() {
		return ctx
	}

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.attrs...),
	}

	if conn != nil {
		opts = append(opts, connectionAttributesFromConfig(conn.Config())...)
	}

	if t.logSQLStatement {
		opts = append(opts, trace.WithAttributes(semconv.DBStatement(data.SQL)))
		if t.includeParams {
			opts = append(opts, trace.WithAttributes(makeParamsAttribute(data.Args)))
		}
	}

	spanName := data.SQL
	if t.trimQuerySpanName {
		spanName = t.sqlOperationName(data.SQL)
	}
	if t.prefixQuerySpanName {
		spanName = "query " + spanName
	}

	ctx, _ = t.tracer.Start(ctx, spanName, opts...)
	return ctx
}

func (t *PgxQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	log.Printf(">>> Query End: tag: %v", data.CommandTag)

	span := trace.SpanFromContext(ctx)

	if data.Err == nil {
		span.SetAttributes(RowsAffectedKey.Int64(data.CommandTag.RowsAffected()))
	} else {
		recordSQLError(span, data.Err)
	}

	span.End()
}

func (t *PgxQueryTracer) TraceConnectStart(ctx context.Context, data pgx.TraceConnectStartData) context.Context {
	log.Println(">>> ConnectionStart:", "connString:", maskPostgresPassword(data.ConnConfig.ConnString()))
	return ctx
}

func (t *PgxQueryTracer) TraceConnectEnd(_ context.Context, data pgx.TraceConnectEndData) {
	if data.Err != nil {
		log.Println(">>> ConnectionEnd:", "Err:", data.Err)
	}
}

// sqlOperationName attempts to get the first 'word' from a given SQL query, which usually
// is the operation name (e.g. 'SELECT').
func (t *PgxQueryTracer) sqlOperationName(stmt string) string {
	// If a custom function is provided, use that. Otherwise, fall back to the
	// default implementation. This allows users to override the default
	// behavior without having to reimplement it.
	if t.spanNameFunc != nil {
		return t.spanNameFunc(stmt)
	}

	parts := strings.Fields(stmt)
	if len(parts) == 0 {
		// Fall back to a fixed value to prevent creating lots of tracing operations
		// differing only by the amount of whitespace in them (in case we'd fall back
		// to the full query or a cut-off version).
		return sqlOperationUnknown
	}
	return strings.ToUpper(parts[0])
}

// connectionAttributesFromConfig returns a slice of SpanStartOptions that contain attributes from the given connection
// config.
func connectionAttributesFromConfig(config *pgx.ConnConfig) []trace.SpanStartOption {
	if config != nil {
		return []trace.SpanStartOption{
			trace.WithAttributes(
				semconv.ClientAddress(config.Host),
				semconv.ClientPort(int(config.Port)),
				semconv.DBUser(config.User),
			),
		}
	}
	return nil
}

func makeParamsAttribute(args []any) attribute.KeyValue {
	ss := make([]string, len(args))
	for i := range args {
		ss[i] = fmt.Sprintf("%+v", args[i])
	}
	return QueryParametersKey.StringSlice(ss)
}

func recordSQLError(span trace.Span, err error) {
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			span.SetAttributes(SQLStateKey.String(pgErr.Code))
		}
	}
}

func findOwnImportedVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		for _, dep := range buildInfo.Deps {
			if dep.Path == tracerName {
				return dep.Version
			}
		}
	}

	return "unknown"
}

func NewQueryTracer(globalAttrs []attribute.KeyValue) pgx.QueryTracer {
	provider := otel.GetTracerProvider()
	return &PgxQueryTracer{
		tracer:              provider.Tracer(tracerName, trace.WithInstrumentationVersion(findOwnImportedVersion())),
		attrs:               globalAttrs,
		trimQuerySpanName:   false,
		spanNameFunc:        nil,
		prefixQuerySpanName: config.OTELPrefixQuerySpanName,
		logSQLStatement:     config.OTELTracerLogSQLStatement,
		includeParams:       config.OTELTracerIncludeParams,
	}
}
