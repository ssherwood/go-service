package app

import (
	"context"
	"errors"
	"github.com/gorilla/mux"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/sdk/trace"
	"locationservice/common"
	"locationservice/infra"
	"locationservice/location"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	serviceName   = "LocationService"
	serverAddress = common.GetEnv("SERVER_ADDRESS", ":8080")
	writeTimeout  = common.GetEnvAsDuration("WRITE_TIMEOUT", 15*time.Second)
	readTimeout   = common.GetEnvAsDuration("READ_TIMEOUT", 10*time.Second)
)

type Application interface {
	Initialize(ctx context.Context) error
	Run()
	Shutdown(ctx context.Context)
}

type LocationApplication struct {
	Server         *http.Server
	Router         *mux.Router
	TracerProvider *trace.TracerProvider
	DB             *pgxpool.Pool
}

func (app *LocationApplication) Initialize(ctx context.Context) error {
	if tp, err := infra.InitTracerProvider(ctx); err != nil {
		return err
	} else {
		app.TracerProvider = tp
	}

	if db, err := infra.InitializeDB(ctx); err != nil {
		return err
	} else {
		app.DB = db
	}

	app.Router = mux.NewRouter()
	app.Router.Use(otelmux.Middleware(serviceName))
	location.RegisterHandlers(app.Router, app.DB)

	app.Server = &http.Server{
		Handler:      app.Router,
		Addr:         serverAddress,
		WriteTimeout: writeTimeout,
		ReadTimeout:  readTimeout,
	}

	return nil
}

func (app *LocationApplication) Run() {

	go func() {
		log.Printf("Starting server on %s", app.Server.Addr)
		if err := app.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Setup signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// create a context with timeout for the shutdown process
	cancelContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.Shutdown(cancelContext); err != nil {
		log.Fatalf("Failed to gracefully shutdown the application: %v", err)
	}

	log.Println("Application stopped gracefully.")
}

// Shutdown - invokes the global shutdown on the app to remove/close open resources
func (app *LocationApplication) Shutdown(ctx context.Context) error {
	log.Println("Shutting down the application...")

	if app.Server != nil {
		if err := app.Server.Shutdown(ctx); err != nil {
			return err // should we really return or just keep truckin'?
		}
	}

	if app.TracerProvider != nil {
		if err := app.TracerProvider.Shutdown(ctx); err != nil {
			return err
		}
	}

	if app.DB != nil {
		// TODO is this graceful for in-flight transactions?
		app.DB.Close()
	}

	return nil
}
