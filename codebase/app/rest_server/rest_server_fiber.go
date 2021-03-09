package restserver

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/soheilhy/cmux"
	"pkg.agungdp.dev/candi/codebase/factory"
	"pkg.agungdp.dev/candi/config/env"
)

type restServerFiber struct {
	serverEngine *fiber.App
	service      factory.ServiceFactory
	httpPort     string
}

// NewServerFiber create new REST server
func NewServerFiber(service factory.ServiceFactory, muxListener cmux.CMux) factory.AppServerFactory {
	server := &restServerFiber{
		serverEngine: fiber.New(),
		service:      service,
		httpPort:     fmt.Sprintf(":%d", env.BaseEnv().HTTPPort),
	}

	// root := server.serverEngine.Group("/", fiberTraceMidd)
	for _, m := range service.GetModules() {
		if h := m.RESTHandler(); h != nil {
			// h.Mount(root)
		}
	}

	fmt.Printf("\x1b[34;1m⇨ HTTP server run at port [::]%s\x1b[0m\n\n", server.httpPort)
	return server
}

func (h *restServerFiber) Serve() {
	h.serverEngine.Listen(h.httpPort)
}

func (h *restServerFiber) Shutdown(ctx context.Context) {
	h.serverEngine.Shutdown()
}
