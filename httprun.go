package httpkit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// RunConfig is a configuration for creating a http Runner.
type RunConfig struct {
	Port            int           // Port to listen to.
	ShutdownTimeout time.Duration // Maximum duration for waiting all active connections to be closed before force close.

	// RequestReadTimeout and RequestWriteTimeout are timeouts for http.Server.
	// These timeouts are used to limit the time spent reading or writing the request body.
	// see: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts.
	RequestReadTimeout  time.Duration // Maximum duration for reading the entire request, including the body.
	RequestWriteTimeout time.Duration // Maximum duration before timing out writes of the response.
}

// Runner is contract for server that can be started, shutdown gracefully and
// force closed if shutdown timeout exceeded.
type Runner interface {
	// ListenAndServe starts listening and serving the server.
	// This method should block until shutdown signal received or failed to start.
	ListenAndServe() error

	// Shutdown gracefully shuts down the server, it will wait for all active connections to be closed.
	Shutdown(ctx context.Context) error

	// Close force closes the server.
	// Close is called when Shutdown timeout exceeded.
	Close() error
}

// RunEvent is a flag to differentiate run events.
type RunEvent uint8

// Sets of run events.
const (
	RunEventInfo   RunEvent = iota // for telling the data is an info.
	RunEventAddr                   // for telling the data is a server address.
	RunEventError                  // for telling the data is an error.
	RunEventSignal                 // for telling the data is a signal received.
)

// String returns the string representation of RunEvent for logging readability.
func (e RunEvent) String() string {
	switch e {
	case RunEventInfo:
		return "info"
	case RunEventAddr:
		return "listening on address"
	case RunEventError:
		return "error occurred"
	case RunEventSignal:
		return "signal received"
	default:
		return "unknown"
	}
}

// GracefulRunner is a wrapper of http.Server that can be shutdown gracefully.
type GracefulRunner struct {
	Runner
	signalListener chan os.Signal
	waitTimeout    time.Duration
	shutdownDone   chan struct{}
	eventListener  func(event RunEvent, data string)
}

// RunOption is the option for customizing the GracefulRunner.
type RunOption func(*GracefulRunner)

// apply applies the option to GracefulRunner.
func (f RunOption) apply(gs *GracefulRunner) { f(gs) }

// NewGracefulRunner wraps a Server with graceful shutdown capability.
// It will listen to SIGINT and SIGTERM signals to initiate shutdown and
// wait for all active connections to be closed. If still active connections
// after wait timeout exceeded, it will force close the server. The default
// wait timeout is 5 seconds.
func NewGracefulRunner(server Runner, opts ...RunOption) *GracefulRunner {
	gs := GracefulRunner{
		Runner:       server,
		shutdownDone: make(chan struct{}),
	}

	for _, opt := range opts {
		opt.apply(&gs)
	}

	// set default options.
	RunOpts.Default().apply(&gs)
	return &gs
}

// ListenAndServe starts listening and serving the server gracefully.
func (s *GracefulRunner) ListenAndServe() error {
	if std, ok := s.Runner.(*http.Server); ok {
		s.eventListener(RunEventAddr, std.Addr)
	} else {
		s.eventListener(RunEventInfo, "server is listening")
	}

	serverErr := make(chan error, 1)
	shutdownCompleted := make(chan struct{})
	// start the original server.
	go func() {
		err := s.Runner.ListenAndServe()
		// if shutdown succeeded, http.ErrServerClosed will be returned.
		if errors.Is(err, http.ErrServerClosed) {
			shutdownCompleted <- struct{}{}
		} else {
			// only send error if it's not http.ErrServerClosed.
			serverErr <- err
			s.eventListener(RunEventError, "server failed")
		}
	}()

	// block until signalListener received or mux failed to start.
	select {
	case sig := <-s.signalListener:
		s.eventListener(RunEventSignal, sig.String())
		s.eventListener(RunEventInfo, "graceful shutdown initiated")

		ctx, cancel := context.WithTimeout(context.Background(), s.waitTimeout)
		defer cancel()

		err := s.Runner.Shutdown(ctx)
		// only force shutdown if deadline exceeded.
		if errors.Is(err, context.DeadlineExceeded) {
			s.eventListener(RunEventInfo, "forced shutdown initiated")
			closeErr := s.Runner.Close()
			if closeErr != nil {
				s.eventListener(RunEventError, "forced shutdown failed")
				return fmt.Errorf("deadline exceeded, force shutdown failed: %w", closeErr)
			}
			// force shutdown succeeded.
			s.eventListener(RunEventInfo, "forced shutdown completed")
			return nil
		}

		// unexpected error.
		if err != nil {
			s.eventListener(RunEventError, "graceful shutdown failed")
			return fmt.Errorf("shutdown failed, signal: %s: %w", sig, err)
		}

		// make sure shutdown completed.
		<-shutdownCompleted
		s.eventListener(RunEventInfo, "graceful shutdown completed")
		return nil
	case err := <-serverErr:
		return fmt.Errorf("server failed to start: %w", err)
	}
}

// runOptionNamespace is type for grouping run options.
type runOptionNamespace int

// RunOpts is the namespace for accessing the Option for customizing the GracefulRunner.
const RunOpts runOptionNamespace = 0

func (runOptionNamespace) Default() RunOption {
	return func(s *GracefulRunner) {
		if s.signalListener == nil {
			RunOpts.Signals(syscall.SIGTERM, syscall.SIGINT).apply(s)
		}

		if s.waitTimeout <= 0 {
			RunOpts.WaitTimeout(5 * time.Second).apply(s)
		}

		if s.eventListener == nil {
			// noop event listener.
			RunOpts.EventListener(func(event RunEvent, data string) {}).apply(s)
		}
	}
}

// Signals sets the signals that will be listened to initiate shutdown.
func (runOptionNamespace) Signals(signals ...os.Signal) RunOption {
	return func(s *GracefulRunner) {
		signalListener := make(chan os.Signal, 1)
		signal.Notify(signalListener, signals...)
		s.signalListener = signalListener
	}
}

// WaitTimeout sets the timeout for waiting active connections to be closed.
func (runOptionNamespace) WaitTimeout(timeout time.Duration) RunOption {
	return func(s *GracefulRunner) { s.waitTimeout = timeout }
}

// EventListener sets the listener that will be called when an event occurred.
func (runOptionNamespace) EventListener(listener func(event RunEvent, data string)) RunOption {
	return func(s *GracefulRunner) { s.eventListener = listener }
}
