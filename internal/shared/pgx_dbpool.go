package shared

import (
	"context"
	"fmt"
	"github.com/ssherwood/ysqlapp/internal/config"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"log/slog"
	"regexp"
	"strings"
)

func InitializeDB(ctx context.Context) (*pgxpool.Pool, error) {
	poolConfig, configErr := pgxPoolConfig()
	if configErr != nil {
		return nil, configErr
	}

	//pgxpool.ParseConfig()
	if dbPool, poolErr := pgxpool.NewWithConfig(ctx, poolConfig); poolErr != nil {
		slog.Error("Unable to create pgx connection pool", config.ErrAttr(poolErr))
		return nil, poolErr
	} else {
		_ = InitPgxPoolMeter(dbPool)
		return dbPool, nil
	}
}

func PingDB(ctx context.Context, pool *pgxpool.Pool) error {
	connection, err := pool.Acquire(ctx)
	if err != nil {
		slog.Error("Unable to acquire a connection from the database pool", "error", err, "config", pool.Config())
		return err
	} else {
		if err = connection.Ping(ctx); err != nil {
			slog.Error("Could not ping database", "error", err, "config", pool.Config())
			return err
		}
	}
	defer connection.Release()

	return nil
}

func pgxPoolConfig() (*pgxpool.Config, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s/%s?%s",
		config.DBUserName, config.DBPassword, config.DBHostname, config.DBDatabase,
		mapToOptions(
			map[string]string{
				"sslmode":           config.DBSSLMode,
				"statement_timeout": config.DBStatementTimeout.String(),
				//"load_balance":      config.DBYSQLLoadBalance,
				//"topology_keys":     config.DBYSQLTopologyKeys,
			},
		),
	)

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		slog.Warn("Failed to parse pgxpool url", config.ErrAttr(err))
		return nil, err
	}

	poolConfig.MaxConns = config.DBMaxConns
	poolConfig.MinConns = config.DBMinConns
	poolConfig.MaxConnLifetime = config.DBMaxConnLifetime
	poolConfig.MaxConnLifetimeJitter = config.DBMaxConnLifetimeJitter
	poolConfig.HealthCheckPeriod = config.DBHealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = config.DBConnectTimeout

	//	poolConfig.
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// your expensive query here:
		_ = conn.QueryRow(ctx, "select pg_sleep(5), * from location loc left join address adr on loc.address_id = adr.id where loc.id='f9654e2a-dc0d-4423-8291-000000004448' and loc.active=true order by loc.id desc limit 1").Scan()
		slog.Warn("AfterConnect")
		return nil
	}

	poolConfig.BeforeAcquire = defaultBeforeAcquireFn()
	poolConfig.AfterRelease = defaultAfterReleaseFn()
	poolConfig.BeforeClose = defaultBeforeCloseFn()

	if config.OTELTracerEnabled {
		poolConfig.ConnConfig.Tracer = NewQueryTracer([]attribute.KeyValue{
			semconv.DBSystemKey.String("yugabytedb"),
			semconv.DBConnectionStringKey.String(maskPostgresPassword(url)),
			semconv.ServerAddress(config.Hostname),
		})
	}

	return poolConfig, nil
}

func defaultBeforeAcquireFn() func(ctx context.Context, c *pgx.Conn) bool {
	return func(ctx context.Context, c *pgx.Conn) bool {
		slog.Debug("Before acquiring a database connection from the pool")

		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			var value string
			_ = c.QueryRow(ctx, "select current_setting('yb_read_from_followers')").Scan(&value)
			slog.Debug("Checking current_setting of yb_read_from_followers", "yb_read_from_followers", value)
		}

		return true
	}
}

func defaultAfterReleaseFn() func(c *pgx.Conn) bool {
	return func(c *pgx.Conn) bool {
		slog.Debug("After releasing database connection back to the pool")

		if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
			var value string
			_ = c.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
			slog.Debug("Checking current_setting of yb_read_from_followers", "yb_read_from_followers", value)
		}

		return true
	}
}

func defaultBeforeCloseFn() func(c *pgx.Conn) {
	return func(c *pgx.Conn) {
		slog.Debug("Closed database connection", "host", c.Config().Host)
	}
}

func mapToOptions(params map[string]string) string {
	var pairs []string
	for key, value := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, "&")
}

func maskPostgresPassword(connURL string) string {
	re := regexp.MustCompile(`(postgres://[^:]+:)([^@]+)(@.+)`)
	return re.ReplaceAllString(connURL, `${1}*****${3}`)
}
