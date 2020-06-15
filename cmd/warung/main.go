package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"agungdwiprasetyo.com/backend-microservices/config"
	"agungdwiprasetyo.com/backend-microservices/internal/services/warung"
	"agungdwiprasetyo.com/backend-microservices/pkg/codebase/app"
	"agungdwiprasetyo.com/backend-microservices/pkg/codebase/factory/base"
)

const (
	serviceName = "warung"
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

	cfg := config.Init(ctx, fmt.Sprintf("cmd/%s/", serviceName))
	defer cfg.Exit(ctx)

	service := warung.NewService(serviceName, base.InitDependency(cfg))
	app.New(service).Run(ctx)
}
