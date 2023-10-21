package httpkit

import "net/http"

// MuxMiddleware is a middleware that applies to route handler.
// It is used to create a middleware that is compatible with httpkit.Handler.
type MuxMiddleware func(Handler) Handler

// Then is a syntactic sugar for applying the middleware to the handler.
// Instead of: m(h), you can write: m.Then(h).
func (m MuxMiddleware) Then(h Handler) Handler { return m(h) }

// NetMiddleware is a middleware that applies to the root handler.
// It is used to create a middleware that is compatible with net/http.
type NetMiddleware func(http.Handler) http.Handler

// Then is a syntactic sugar for applying the middleware to the handler.
// Instead of: m(h), you can write: m.Then(h).
func (n NetMiddleware) Then(h http.Handler) http.Handler { return n(h) }

// ReduceMuxMiddleware reduces a set of mux middlewares into a single mux
// middleware. For example:
//
//	ReduceMuxMiddleware(m1, m2, m3).Then(h)
//	will be equivalent to:
//	m1(m2(m3(h)))
func ReduceMuxMiddleware(middlewares ...MuxMiddleware) MuxMiddleware {
	return reduceMuxMiddleware(middlewares)
}

// ReduceNetMiddleware reduces a set of net middlewares into a single net
// middleware. For example:
//
//	ReduceNetMiddleware(m1, m2, m3).Then(h)
//	will be equivalent to:
//	m1(m2(m3(h)))
func ReduceNetMiddleware(middlewares ...NetMiddleware) NetMiddleware {
	return reduceNetMiddleware(middlewares)
}

func reduceMuxMiddleware(middlewares []MuxMiddleware) MuxMiddleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i].Then(next)
		}
		return next
	}
}

func reduceNetMiddleware(middlewares []NetMiddleware) NetMiddleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i].Then(next)
		}
		return next
	}
}
