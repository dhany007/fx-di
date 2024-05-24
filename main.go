package main

import (
	"context"
	"io"
	"net"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

// NewHTTPServer build a HTTP server that will begin serving requests
// when the Fx application starts
func NewHTTPServer(lc fx.Lifecycle, mux *http.ServeMux, log *zap.Logger) *http.Server {
	srv := &http.Server{Addr: ":8080", Handler: mux}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}

			log.Info("Starting HTTP server at", zap.String("addr", srv.Addr))

			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})

	return srv
}

type EchoHandler struct {
	log *zap.Logger
}

func NewEchoHandler(
	log *zap.Logger,
) *EchoHandler {
	return &EchoHandler{
		log: log,
	}
}

// Route is an http.Handler that knows the mux pattern
// under which it will be registered.
type Route interface {
	http.Handler

	// Pattern reports the path at which this is registered.
	Pattern() string
}

func (e *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := io.Copy(w, r.Body); err != nil {
		e.log.Warn("Failed to handle request: ", zap.Error(err))
	}
}

func (e *EchoHandler) Pattern() string {
	return "/echo"
}

// NewServeMux builds a ServeMux that will route requests
// to the given Route.
func NewServeMux(route Route) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(route.Pattern(), route)
	return mux
}

func main() {
	fx.New(
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Provide(
			NewHTTPServer,
			NewServeMux,
			fx.Annotate(
				NewEchoHandler,
				fx.As(new(Route)), //cast its result to that interface
			),
			zap.NewExample, // in production should use zap.NewProduction
		), // provide: register function

		fx.Invoke(func(*http.Server) {}), // invoke: run function
	).Run()
}
