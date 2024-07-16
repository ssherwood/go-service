package main

import (
	"context"
	"locationservice/app"
	"log"
)

func main() {
	locationApp := &app.LocationApplication{}

	if err := locationApp.Initialize(context.Background()); err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	locationApp.Run()
}
