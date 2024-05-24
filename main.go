package main

import (
	"context"
	"fmt"
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

type HelloHandler struct {
	log *zap.Logger
}

func NewHelloHandler(
	log *zap.Logger,
) *HelloHandler {
	return &HelloHandler{
		log: log,
	}
}

func (h *HelloHandler) Pattern() string {
	return "/hello"
}

func (h *HelloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("failed to read request", zap.Error(err))
		http.Error(w, "internal server error", http.StatusBadRequest)
		return
	}

	_, err = fmt.Fprintf(w, "hello, %s\n", body)
	if err != nil {
		h.log.Error("failed to write response", zap.Error(err))
		http.Error(w, "internal server error", http.StatusBadRequest)
		return
	}
}

// NewServeMux builds a ServeMux that will route requests
// to the given Route.
func NewServeMux(route1, route2 Route) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(route1.Pattern(), route1)
	mux.Handle(route2.Pattern(), route2)
	return mux
}

func main() {
	fx.New(
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Provide(
			NewHTTPServer,
			fx.Annotate(
				NewServeMux,
				fx.ParamTags(`name:"echo"`, `name:"hello"`),
			),
			fx.Annotate(
				NewEchoHandler,
				fx.As(new(Route)), //cast its result to that interface
				fx.ResultTags(`name:"echo"`),
			),
			fx.Annotate(
				NewHelloHandler,
				fx.As(new(Route)),
				fx.ResultTags(`name:"hello"`),
			),
			zap.NewExample, // in production should use zap.NewProduction
		), // provide: register function

		fx.Invoke(func(*http.Server) {}), // invoke: run function
	).Run()
}
