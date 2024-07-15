package main

import (
	"context"
	"github.com/gorilla/mux"
	middleware "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"locationservice/database"
	"locationservice/handlers"
	"locationservice/telemetry"
	"log"
	"net/http"
)

func main() {
	ctx := context.Background()

	// initialize OTEL tracing
	tracerProvider, err := telemetry.InitTracing(ctx)
	if err != nil {
		log.Fatalf("Could not initialize tracer: %v", err)
	}
	defer func() {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()

	dbPool, err := database.ConnectDB(ctx)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer dbPool.Close()

	router := mux.NewRouter()
	router.Use(middleware.Middleware("location-service"))
	handlers.RegisterLocationHandlers(router, dbPool)

	log.Println("Server running on port 8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}
