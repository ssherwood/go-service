package main

import (
	"context"
	"github.com/ssherwood/ysqlapp/internal/app"
	"github.com/ssherwood/ysqlapp/internal/config"
	"log/slog"
)

func main() {
	locationApp := &app.LocationApplication{}

	if err := locationApp.Initialize(context.Background()); err != nil {
		slog.Error("Failed to initialize application", config.ErrAttr(err))
	}

	locationApp.Run()
}
