package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgconn"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"log"
	"runtime/debug"
	"strings"
	"time"
)

const (
	tracerName          = "github.com/exaring/otelpgx"
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

// connectionAttributesFromConfig returns a slice of SpanStartOptions that contain
// attributes from the given connection config.
func connectionAttributesFromConfig(config *pgx.ConnConfig) []trace.SpanStartOption {
	if config != nil {
		return []trace.SpanStartOption{
			trace.WithAttributes(
				semconv.NetPeerName(config.Host),
				semconv.NetPeerPort(int(config.Port)),
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

type LogTracer struct {
	tracer              trace.Tracer
	attrs               []attribute.KeyValue
	trimQuerySpanName   bool
	spanNameFunc        SpanNameFunc
	prefixQuerySpanName bool
	logSQLStatement     bool
	includeParams       bool
}

func (t *LogTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
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

	log.Println(">>> Executing command:", "sql:", data.SQL, "args:", data.Args)
	return ctx
}

func recordError(span trace.Span, err error) {
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			span.SetAttributes(SQLStateKey.String(pgErr.Code))
		}
	}
}

func (t *LogTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	recordError(span, data.Err)

	if data.Err == nil {
		span.SetAttributes(RowsAffectedKey.Int64(data.CommandTag.RowsAffected()))
	}

	span.End()

	log.Println(">>> Finishing command:", "tag:", data.CommandTag)
}

func (t *LogTracer) TraceConnectStart(ctx context.Context, data pgx.TraceConnectStartData) context.Context {
	log.Println(">>> ConnectionStart:", "connString:", data.ConnConfig.ConnString())
	return ctx
}

func (t *LogTracer) TraceConnectEnd(_ context.Context, data pgx.TraceConnectEndData) {
	if data.Err != nil {
		log.Println(">>> ConnectionEnd:", "Err:", data.Err)
	}
}

// sqlOperationName attempts to get the first 'word' from a given SQL query, which usually
// is the operation name (e.g. 'SELECT').
func (t *LogTracer) sqlOperationName(stmt string) string {
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

func convertMapToString(params map[string]string) string {
	var pairs []string
	for key, value := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, "&")
}

func Config() *pgxpool.Config {

	configOptions := map[string]string{
		"load_balance":  "false",
		"topology_keys": "gcp.us-east1.*:1,gcp.us-central1.*:2,gcp.us-west1.*:3",
	}
	//fmt.Println(convertMapToString(configOptions))

	url := fmt.Sprintf("postgres://%s:%s@%s/%s?%s", "yugabyte", "", "127.0.0.1:5433,127.0.0.2:5433,127.0.0.3:5433", "yugabyte", convertMapToString(configOptions))
	dbConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatal("Failed to create a config, error: ", err)
	}

	dbConfig.MaxConns = 10
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour * 4
	dbConfig.MaxConnLifetimeJitter = time.Minute * 15
	//dbConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	dbConfig.HealthCheckPeriod = time.Minute * 10
	dbConfig.ConnConfig.ConnectTimeout = time.Second * 5

	traceProvider := otel.GetTracerProvider()
	tracer := LogTracer{
		tracer: traceProvider.Tracer(tracerName, trace.WithInstrumentationVersion(findOwnImportedVersion())),
		attrs: []attribute.KeyValue{
			semconv.DBSystemKey.String("yugabytedb"),
			semconv.DBConnectionStringKey.String(url),
		},
		trimQuerySpanName:   false,
		spanNameFunc:        nil,
		prefixQuerySpanName: true,
		logSQLStatement:     true,
		includeParams:       false,
	}
	dbConfig.ConnConfig.Tracer = &tracer

	dbConfig.BeforeAcquire = func(ctx context.Context, c *pgx.Conn) bool {
		log.Println("Before acquiring the connection pool to the database!!")

		var value string
		_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}

	dbConfig.AfterRelease = func(c *pgx.Conn) bool {
		log.Println("After releasing the connection pool to the database!!")

		var value string
		_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
		log.Println("yb_read_from_followers is", value)

		return true
	}

	dbConfig.BeforeClose = func(c *pgx.Conn) {
		log.Println("Closed the connection pool to the database!!")
	}

	return dbConfig
}

func ConnectDB(ctx context.Context) (*pgxpool.Pool, error) {
	dbPool, err := pgxpool.NewWithConfig(ctx, Config())
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
		return nil, err
	}

	return dbPool, nil
}
