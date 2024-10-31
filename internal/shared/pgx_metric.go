package shared

import (
	"context"
	"github.com/ssherwood/locationservice/internal/config"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"log/slog"
)

// InitPgxPoolMeter
// given a pgxpool.Pool, build a databaseMeter callback for the available statistics available.
func InitPgxPoolMeter(dbPool *pgxpool.Pool) error {
	var databaseMeter = otel.Meter("github.com/yugabyte/pgx/v5/pgxpool",
		metric.WithInstrumentationAttributes(
			semconv.ServiceName(config.ServiceName),
		),
	)

	idleConns, _ := databaseMeter.Int64ObservableGauge("pgxpool.idleConns")
	totalConns, _ := databaseMeter.Int64ObservableGauge("pgxpool.totalConns")
	acquireCount, _ := databaseMeter.Int64ObservableCounter("pgxpool.acquireCount")
	newConnsCount, _ := databaseMeter.Int64ObservableCounter("pgxpool.newConnsCount")
	maxLifetimeDestroyCount, _ := databaseMeter.Int64ObservableCounter("pgxpool.maxLifetimeDestroyCount")
	acquireDuration, _ := databaseMeter.Int64ObservableGauge("pgxpool.acquireDuration", metric.WithUnit("ms"))
	// TODO finish remaining metrics...

	_, err := databaseMeter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			dbStats := dbPool.Stat()
			o.ObserveInt64(idleConns, int64(dbStats.IdleConns()))
			o.ObserveInt64(totalConns, int64(dbStats.TotalConns()))
			o.ObserveInt64(acquireCount, dbStats.AcquireCount())
			o.ObserveInt64(newConnsCount, dbStats.NewConnsCount())
			o.ObserveInt64(maxLifetimeDestroyCount, dbStats.MaxLifetimeDestroyCount())
			o.ObserveInt64(acquireDuration, dbStats.AcquireDuration().Milliseconds())
			return nil
		},
		idleConns, totalConns, acquireCount, newConnsCount, maxLifetimeDestroyCount, acquireDuration,
	)
	if err != nil {
		slog.Error("failed to register pgxpool stats", config.ErrAttr(err))
		return err
	}

	return nil
}
