# HTTP Kit for Go Backend Engineers

This kit is my personal utility, and I have used it in both my professional and hobby projects. I hope you will find it useful as well. I would be happy to receive your feedback and suggestions.

## Features

- [x] Modifies the [httprouter](github.com/julienschmidt/httprouter) to allow the handler to return errors.
- [x] HTTP Server with Graceful Shutdown.
- [x] Helpers for creating middleware for both net/http and the modified httprouter.
- [x] Helpers for request decoding and response encoding.
- [x] A middleware for recording logs. This middleware can be used to implement the [Canonical Log Line](https://stripe.com/blog/canonical-log-lines).

## TODOS

- [ ] Add a Content Negotiation Middleware
- [ ] Add more encoders and decoders.
- [ ] Add more common middlewares.
- [ ] Add Open Telemetry support.


## Examples

### Creating a Server with Graceful Shutdown

```go
package main

import (
	"fmt"
	"github.com/josestg/httpkit"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"
)

func main() {
	log := slog.Default()

	mux := httpkit.NewServeMux(
		// Register an error handler that will be called when there is still unresolved error after all the middlewares
		// and the handler have been called. We can think this as the last hope when all else fails.
		httpkit.Opts.LastResortErrorHandler(LastResortErrorHandler(log)),

		// Set the global middlewares for the mux. These middlewares will be applied to all the handlers that are
		// registered to this mux.
		//
		// For middlewares that are applied to a specific handler, `mux.Route` takes variadic `httpkit.MuxMiddleware`
		// as the last argument, which will be applied to that handler only.
		httpkit.Opts.Middleware(GlobalMiddleware()),

		// And more options are available under `httpkit.Opts` namespace.
	)

	// Using `Route` instead of `HandleFunc` will be more convenient when working with Swagger docs because the swagger
	// docs and the route definition will be close to each other. Which will make it easier to keep them in sync.
	mux.Route(httpkit.Route{
		Method: http.MethodPost,
		Path:   "/",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			data := struct {
				Message string `json:"message"`
			}{}

			if err := httpkit.ReadJSON(r.Body, &data); err != nil {
				return fmt.Errorf("could not read json: %w", err)
			}

			// do something with the data.
			data.Message += " --updated"
			return httpkit.WriteJSON(w, data, http.StatusOK)
		},
	})

	// this middleware will be applied to the router itself. Meaning that it will be called before and after
	// the router calls the handler.
	mid := httpkit.ReduceNetMiddleware(
		// record both request and response body, along with latency and status code.
		httpkit.LogEntryRecorder,

		// another example of a middleware that will be applied to the router itself.
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Info("before router")
				next.ServeHTTP(w, r)
				entry, ok := httpkit.GetLogEntry(w)
				if ok {
					log.Info("after router", "method", r.Method, "path", r.URL.Path,
						"status", entry.StatusCode, "latency", time.Duration(entry.RespondedAt-entry.RequestedAt))
				}
			})
		},
	)

	// apply the middleware to the router.
	handler := mid.Then(mux)

	srv := http.Server{
		Addr:         "0.0.0.0:8080",
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// create a graceful server runner.
	run := httpkit.NewGracefulRunner(
		&srv,
		httpkit.RunOpts.WaitTimeout(10*time.Second),              // wait for 10 seconds before force shutdown.
		httpkit.RunOpts.Signals(syscall.SIGINT, syscall.SIGTERM), // listen to SIGINT and SIGTERM signals.
		httpkit.RunOpts.EventListener(func(evt httpkit.RunEvent, data string) { // listen to events and log them.
			switch evt {
			default:
				log.Info(data)
			case httpkit.RunEventAddr:
				log.Info("http server listening", "addr", data)
			case httpkit.RunEventSignal:
				log.Info("http server received shutdown signal", "signal", data)
			}
		}),
	)

	if err := run.ListenAndServe(); err != nil {
		log.Error("http server error", "error", err)
		os.Exit(1)
	}
}

func LastResortErrorHandler(log *slog.Logger) httpkit.LastResortErrorHandler {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error("last resort error handler", "error", err)
		data := map[string]string{
			"message": http.StatusText(http.StatusInternalServerError),
		}
		if wErr := httpkit.WriteJSON(w, data, http.StatusInternalServerError); wErr != nil {
			log.Error("could not write json", "unresolved_error", err, "write_error", wErr)
		}
	}
}

func GlobalMiddleware() httpkit.MuxMiddleware {
	return httpkit.ReduceMuxMiddleware(
		globalMiddlewareOne,
		globalMiddlewareTwo,
	)
}

func globalMiddlewareOne(next httpkit.Handler) httpkit.Handler {
	return httpkit.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		// do something before calling the next handler.
		err := next.ServeHTTP(w, r)
		// do something after calling the next handler.
		return err
	})
}

func globalMiddlewareTwo(next httpkit.Handler) httpkit.Handler {
	return httpkit.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		// do something before calling the next handler.
		err := next.ServeHTTP(w, r)
		// do something after calling the next handler.
		return err
	})
}

```