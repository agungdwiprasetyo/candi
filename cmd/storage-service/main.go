package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"agungdwiprasetyo.com/backend-microservices/config"
	"agungdwiprasetyo.com/backend-microservices/internal/app"
	service "agungdwiprasetyo.com/backend-microservices/internal/services/storage-service"
)

const (
	serviceName = "storage-service"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer func() {
		cancel()
		if r := recover(); r != nil {
			fmt.Printf("Failed to start %s service: %v\n", serviceName, r)
			fmt.Printf("Stack trace: \n%s\n", debug.Stack())
		}
	}()

	cfg := config.Init(ctx, "cmd/"+serviceName)
	defer cfg.Exit(ctx)

	srv := service.NewService(cfg, serviceName)
	app.New(srv).Run(ctx)
}
