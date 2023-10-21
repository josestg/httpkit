package httpkit

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type (
	// Param is alias of httprouter.Param.
	Param = httprouter.Param

	// Params is alias of httprouter.Params.
	Params = httprouter.Params
)

// PathParams gets the path variables from the request.
func PathParams(r *http.Request) Params {
	return httprouter.ParamsFromContext(r.Context())
}

// Handler is modified version of http.Handler.
type Handler interface {
	// ServeHTTP is just like http.Handler.ServeHTTP, but it returns an error.
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

// HandlerFunc is a function that implements Handler.
// It is used to create a Handler from an ordinary function.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// ServeHTTP implements Handler.
func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) error { return f(w, r) }

// LastResortErrorHandler is the error handler that is called if after all middlewares,
// there is still an error occurs.
type LastResortErrorHandler func(http.ResponseWriter, *http.Request, error)

// Route is used to register a new handler to the ServeMux.
type Route struct {
	Method  string
	Path    string
	Handler HandlerFunc
}

// ServeMux is a wrapper of httprouter.Router with modified Handler.
// Instead of http.Handler, it uses Handler, which returns an error. This modification is used to simplify logic for
// creating a centralized error handler and logging.
//
// The ServeMux also supports MuxMiddleware, which is a middleware that wraps the Handler for all routes. Since the
// ServeMux also implements http.Handler, the NetMiddleware can be used to create middleware that will be executed
// before the ServeMux middleware.
//
// The ServeMux only exposes 3 methods: Route, Handle, and ServeHTTP, which are more simple than the original.
type ServeMux struct {
	core *httprouter.Router
	conf *MuxConfig
	midl MuxMiddleware
}

// NewServeMux creates a new ServeMux with given options.
// If no option is given, the Default option is applied.
func NewServeMux(opts ...MuxOption) *ServeMux {
	mux := ServeMux{conf: &MuxConfig{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}}

	for i := range opts {
		opts[i].applyTo(&mux)
	}

	// apply default config for unset options.
	Opts.Default().applyTo(&mux)
	mux.core = &httprouter.Router{
		RedirectTrailingSlash:  mux.conf.RedirectTrailingSlash,
		RedirectFixedPath:      mux.conf.RedirectFixedPath,
		HandleMethodNotAllowed: mux.conf.HandleMethodNotAllowed,
		HandleOPTIONS:          mux.conf.HandleOPTIONS,
		GlobalOPTIONS:          mux.conf.GlobalOPTIONS,
		NotFound:               mux.conf.NotFound,
		MethodNotAllowed:       mux.conf.MethodNotAllowed,
		PanicHandler:           mux.conf.PanicHandler,
	}
	return &mux
}

// Route is a syntactic sugar for Handle(method, path, handler) by using Route struct.
// This route also accepts variadic MuxMiddleware, which is applied to the route handler.
func (mux *ServeMux) Route(r Route, mid ...MuxMiddleware) {
	mux.Handle(r.Method, r.Path, reduceMuxMiddleware(mid).Then(r.Handler))
}

// Handle registers a new request handler with the given method and path.
func (mux *ServeMux) Handle(method, path string, handler Handler) {
	mux.core.HandlerFunc(method, path, func(w http.ResponseWriter, r *http.Request) {
		err := mux.midl.Then(handler).ServeHTTP(w, r)
		if err != nil {
			mux.conf.LastResortErrorHandler(w, r, err)
		}
	})
}

// ServeHTTP satisfies http.Handler.
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux.core.ServeHTTP(w, r)
}

// MuxConfig is the configuration for the underlying httprouter.Router.
type MuxConfig struct {
	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 307 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterward the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 307 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// If enabled, the router checks if another method is allowed for the
	// current route, if the current request can not be routed.
	// If this is the case, the request is answered with 'Method Not Allowed'
	// and HTTP status code 405.
	// If no other Method is allowed, the request is delegated to the NotFound
	// handler.
	HandleMethodNotAllowed bool

	// If enabled, the router automatically replies to OPTIONS requests.
	// Custom OPTIONS handlers take priority over automatic replies.
	HandleOPTIONS bool

	// An optional http.Handler that is called on automatic OPTIONS requests.
	// The handler is only called if HandleOPTIONS is true and no OPTIONS
	// handler for the specific path was set.
	// The "Allowed" header is set before calling the handler.
	GlobalOPTIONS http.Handler

	// Configurable http.Handler which is called when no matching route is
	// found.
	NotFound http.Handler

	// Configurable http.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// The "Allow" header with allowed request methods is set before the handler
	// is called.
	MethodNotAllowed http.Handler

	// Function to handle panics recovered from http handlers.
	// It should be used to generate an error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecoverable panics.
	PanicHandler func(http.ResponseWriter, *http.Request, any)

	// LastResortErrorHandler is the error handler that is called if after all middlewares,
	// there is still an error occurs. This handler is used to catch errors that are not handled by the middlewares.
	//
	// This handler is not part of the httprouter.Router, it is used by the ServeMux.
	LastResortErrorHandler LastResortErrorHandler
}

// MuxOption is an option for customizing the ServeMux.
type MuxOption func(mux *ServeMux)

// applyTo applies the option to the ServeMux.
func (f MuxOption) applyTo(mux *ServeMux) { f(mux) }

// muxOptionNamespace is an internal type for grouping options.
type muxOptionNamespace int

// Opts is a namespace for accessing options.
const Opts muxOptionNamespace = 0

// Default configures the ServeMux with default options.
func (muxOptionNamespace) Default() MuxOption {
	return func(mux *ServeMux) {
		defaults := make([]MuxOption, 0, 5) // at most 5 default options.
		if mux.conf.LastResortErrorHandler == nil {
			defaults = append(defaults, Opts.LastResortErrorHandler(DefaultHandler.LastResortError))
		}

		if mux.conf.NotFound == nil {
			defaults = append(defaults, Opts.NotFoundHandler(DefaultHandler.NotFound()))
		}

		if mux.conf.MethodNotAllowed == nil {
			defaults = append(defaults, Opts.MethodNotAllowedHandler(DefaultHandler.MethodNotAllowed()))
		}

		if mux.conf.PanicHandler == nil {
			defaults = append(defaults, Opts.PanicHandler(DefaultHandler.Panic))
		}

		if mux.midl == nil {
			// add an identity middleware, to avoid nil pointer dereference check.
			defaults = append(defaults, Opts.Middleware(func(h Handler) Handler { return h }))
		}

		for i := range defaults {
			defaults[i].applyTo(mux)
		}
	}
}

// RedirectTrailingSlash enables/disables automatic redirection if the current route can't be matched but a
// handler for the path with (without) the trailing slash exists. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.RedirectTrailingSlash
func (muxOptionNamespace) RedirectTrailingSlash(enabled bool) MuxOption {
	return func(mux *ServeMux) { mux.conf.RedirectTrailingSlash = enabled }
}

// RedirectFixedPath if enabled, the router tries to fix the current request path, if no
// handle is registered for it. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.RedirectFixedPath
func (muxOptionNamespace) RedirectFixedPath(enabled bool) MuxOption {
	return func(mux *ServeMux) { mux.conf.RedirectFixedPath = enabled }
}

// HandleMethodNotAllowed if enabled, the router checks if another method is allowed for the
// current route, if the current request can not be routed. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.HandleMethodNotAllowed
func (muxOptionNamespace) HandleMethodNotAllowed(enabled bool) MuxOption {
	return func(mux *ServeMux) { mux.conf.HandleMethodNotAllowed = enabled }
}

// HandleOption if enabled, the router automatically replies to OPTIONS requests.
// Custom OPTIONS handlers take priority over automatic replies. Default enabled.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.HandleOPTIONS
func (muxOptionNamespace) HandleOption(enabled bool) MuxOption {
	return func(mux *ServeMux) { mux.conf.HandleOPTIONS = enabled }
}

// GlobalOptionHandler sets the global OPTIONS handler.
// The handler is only called if HandleOPTIONS is true and no OPTIONS handler for the specific path was set.
//
// see: https://godoc.org/github.com/julienschmidt/httprouter#Router.GlobalOPTIONS
func (muxOptionNamespace) GlobalOptionHandler(handler http.Handler) MuxOption {
	return func(mux *ServeMux) { mux.conf.GlobalOPTIONS = handler }
}

// NotFoundHandler sets the handler that is called when no matching route is found.
// If it is not set, DefaultHandler.NotFound is used.
func (muxOptionNamespace) NotFoundHandler(handler http.Handler) MuxOption {
	return func(mux *ServeMux) { mux.conf.NotFound = handler }
}

// MethodNotAllowedHandler sets the handler that is called when a request
// cannot be routed and HandleMethodNotAllowed is true. If it is not set, DefaultHandler.MethodNotAllowed is used.
func (muxOptionNamespace) MethodNotAllowedHandler(handler http.Handler) MuxOption {
	return func(mux *ServeMux) { mux.conf.MethodNotAllowed = handler }
}

// PanicHandler sets the handler that is called when a panic occurs.
// If no handler is set, the DefaultHandler.LastResortError is used.
func (muxOptionNamespace) PanicHandler(handler func(http.ResponseWriter, *http.Request, any)) MuxOption {
	return func(mux *ServeMux) { mux.conf.PanicHandler = handler }
}

// LastResortErrorHandler sets the handler that is called if after all middlewares,
// there is still an error occurs.
// This handler is used to catch errors that are not handled by the middlewares.
func (muxOptionNamespace) LastResortErrorHandler(handler LastResortErrorHandler) MuxOption {
	return func(mux *ServeMux) { mux.conf.LastResortErrorHandler = handler }
}

// Middleware sets the middleware for all routes in the ServeMux.
// This middleware is called before the request is received by the Route Handler, that means if route has specific
// middleware, it will be called after this middleware. In other words, this middleware is the outermost middleware.
func (muxOptionNamespace) Middleware(m MuxMiddleware) MuxOption {
	return func(mux *ServeMux) { mux.midl = m }
}

// defaultHandlerNamespace is an internal type for grouping default handlers.
type defaultHandlerNamespace int

// DefaultHandler is a namespace for accessing default handlers.
const DefaultHandler defaultHandlerNamespace = 0

// LastResortError is the default last resort error handler.
func (defaultHandlerNamespace) LastResortError(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "default last resort error handler: method: %s, path: %s, error: %v", r.Method, r.URL.Path, err)
}

// NotFound is the default not found handler.
func (defaultHandlerNamespace) NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "default not found handler: method: %s, path: %s", r.Method, r.URL.Path)
	}
}

// MethodNotAllowed is the default method not allowed handler.
func (defaultHandlerNamespace) MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = fmt.Fprintf(w, "default method not allowed handler: method: %s, path: %s", r.Method, r.URL.Path)
	}
}

// Panic is the default panic handler.
func (defaultHandlerNamespace) Panic(w http.ResponseWriter, r *http.Request, v any) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "default panic handler: method: %s, path: %s, error: %v", r.Method, r.URL.Path, v)
}
