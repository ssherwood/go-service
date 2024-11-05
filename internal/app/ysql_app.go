package app

import (
	"context"
	"errors"
	"github.com/gorilla/mux"
	"github.com/ssherwood/ysqlapp/internal/config"
	"github.com/ssherwood/ysqlapp/internal/location"
	"github.com/ssherwood/ysqlapp/internal/shared"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/log"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Application interface {
	Initialize(ctx context.Context) error
	Run()
	Shutdown(ctx context.Context)
}

type LocationApplication struct {
	Server          *http.Server
	Router          *mux.Router
	TracerProvider  *trace.TracerProvider
	MetricsProvider *metricsdk.MeterProvider
	LoggerProvider  *log.LoggerProvider
	DB              *pgxpool.Pool
	TestCtr         metric.Int64Counter
}

func (app *LocationApplication) Initialize(ctx context.Context) error {
	//if lp, err := shared.InitializeLoggingProvider(ctx); err != nil {
	//	return err
	//} else {
	//	app.LoggerProvider = lp
	//
	//	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	//	otelHandler := shared.NewOTLPSlogHandler(consoleHandler, lp)
	//	logger := slog.New(otelHandler)
	//
	//	otelLogger := slog.New(otelslog.NewOtelHandler(otel.GetLoggerProvider(), &otelslog.HandlerOptions{}))
	//	slog.SetDefault(otelLogger)
	//}

	//if tp, err := shared.InitTracerProvider(ctx); err != nil {
	//	return err
	//} else {
	//	app.TracerProvider = tp
	//}

	//if mp, err := shared.InitializeMetricProvider(ctx); err != nil {
	//	return err
	//} else {
	//	app.MetricsProvider = mp
	//}

	//if err := app.metricTest(ctx); err != nil {
	//	return err
	//}

	if db, err := shared.InitializeDB(ctx); err != nil {
		return err
	} else {
		app.DB = db

		// force establishing at least one valid connection
		if err = shared.PingDB(ctx, db); err != nil {
			return err
		}
	}

	app.Router = mux.NewRouter()
	app.Router.Use(otelmux.Middleware(config.ServiceName))

	locationRepository := location.NewRepository(app.DB)
	locationService := location.NewService(locationRepository)
	_ = location.NewHandler(app.Router, locationService, app.DB)

	app.Server = &http.Server{
		Handler:      app.Router,
		Addr:         config.ServerAddress,
		WriteTimeout: config.ServerWriteTimeout,
		ReadTimeout:  config.ServerReadTimeout,
		//ErrorLog:     slog.Default(),
	}

	return nil
}

// this is just a test metric for now
func (app *LocationApplication) metricTest(ctx context.Context) error {
	meter := app.MetricsProvider.Meter("foo")
	if c, err := meter.Int64Counter("test.ctr",
		metric.WithDescription("The number of calls to GetLocation"),
		metric.WithUnit("{test}")); err != nil {
		return err
	} else {
		app.TestCtr = c
	}
	app.TestCtr.Add(ctx, 1, metric.WithAttributes(attribute.Int("test.value", 1)))
	return nil
}

func (app *LocationApplication) Run() {

	go func() {
		slog.Info("Starting application", config.SlogServiceName, config.SlogServiceAddress)
		if err := app.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("Failed to start application", config.SlogServiceName, config.ErrAttr(err))
		}
	}()

	// Setup signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// create a context with timeout for the shutdown process
	cancelContext, cancelFn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFn()

	if err := app.Shutdown(cancelContext); err != nil {
		slog.Info("Failed to gracefully shutdown", config.SlogServiceName, config.ErrAttr(err))
	}

	slog.Info("Application stopped.", config.SlogServiceName)
}

// Shutdown - invokes the global shutdown on the app to remove/close open resources
func (app *LocationApplication) Shutdown(ctx context.Context) error {
	slog.Info("Application shutting down...", config.SlogServiceName)

	// TODO should we do these in parallel with goroutines?
	if app.Server != nil {
		if err := app.Server.Shutdown(ctx); err != nil {
			slog.Warn("Unable to shutdown HTTP server", config.SlogServiceName, config.ErrAttr(err))
		}
	}

	if app.MetricsProvider != nil {
		if err := app.MetricsProvider.Shutdown(ctx); err != nil {
			slog.Warn("Unable to shutdown OTEL metrics provider", config.SlogServiceName, config.ErrAttr(err))
		}
	}

	if app.TracerProvider != nil {
		if err := app.TracerProvider.Shutdown(ctx); err != nil {
			slog.Warn("Unable to shutdown OTEL tracer provider", config.ErrAttr(err))
		}
	}

	if app.LoggerProvider != nil {
		if err := app.LoggerProvider.Shutdown(ctx); err == nil {
			slog.Warn("Unable to shutdown OTEL logger provider", config.ErrAttr(err))
		}
	}

	if app.DB != nil {
		// TODO is this graceful for in-flight transactions?
		app.DB.Close()
	}

	return nil
}
