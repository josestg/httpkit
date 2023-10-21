package httpkit

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultHandlerNamespace_LastResortError(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	err := errors.New("some error")
	DefaultHandler.LastResortError(res, req, err)
	expectTrue(t, res.Code == 500)
}

func TestDefaultHandlerNamespace_MethodNotAllowed(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	DefaultHandler.MethodNotAllowed().ServeHTTP(res, req)
	expectTrue(t, res.Code == 405)
}

func TestDefaultHandlerNamespace_NotFound(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	DefaultHandler.NotFound().ServeHTTP(res, req)
	expectTrue(t, res.Code == 404)
}

func TestDefaultHandlerNamespace_PanicHandler(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	DefaultHandler.Panic(res, req, "any value")
	expectTrue(t, res.Code == 500)
}

func TestHandlerFunc_ServeHTTP(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(201)
		return nil
	})

	err := h.ServeHTTP(res, req)
	expectTrue(t, err == nil)
	expectTrue(t, res.Code == 201)
}

func TestNewServeMux_Default(t *testing.T) {
	mux := NewServeMux()
	expectTrue(t, mux.conf.RedirectTrailingSlash)
	expectTrue(t, mux.conf.RedirectFixedPath)
	expectTrue(t, mux.conf.HandleMethodNotAllowed)
	expectTrue(t, mux.conf.HandleOPTIONS)
	expectTrue(t, mux.conf.GlobalOPTIONS == nil)
	expectTrue(t, mux.conf.NotFound != nil)
	expectTrue(t, mux.conf.MethodNotAllowed != nil)
	expectTrue(t, mux.conf.PanicHandler != nil)
	expectTrue(t, mux.conf.LastResortErrorHandler != nil)
	expectTrue(t, mux.midl != nil)
	expectTrue(t, mux.core != nil)
}

func TestNewServeMux_CustomOptions(t *testing.T) {
	mux := NewServeMux(
		Opts.RedirectFixedPath(false),
		Opts.RedirectTrailingSlash(false),
		Opts.HandleMethodNotAllowed(false),
		Opts.HandleOption(false),
		Opts.GlobalOptionHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
	)
	expectFalse(t, mux.conf.RedirectTrailingSlash)
	expectFalse(t, mux.conf.RedirectFixedPath)
	expectFalse(t, mux.conf.HandleMethodNotAllowed)
	expectFalse(t, mux.conf.HandleOPTIONS)
	expectTrue(t, mux.conf.GlobalOPTIONS != nil)
	expectTrue(t, mux.conf.NotFound != nil)
	expectTrue(t, mux.conf.MethodNotAllowed != nil)
	expectTrue(t, mux.conf.PanicHandler != nil)
	expectTrue(t, mux.conf.LastResortErrorHandler != nil)
	expectTrue(t, mux.midl != nil)
	expectTrue(t, mux.core != nil)
}

func TestServeMux_Route(t *testing.T) {
	mux := NewServeMux()
	mux.Route(Route{
		Method: "POST",
		Path:   "/data",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(201)
			return nil
		},
	})

	t.Run("POST /data: expect 201", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/data", nil)
		mux.ServeHTTP(res, req)
		expectTrue(t, res.Code == 201)
	})

	t.Run("GET /data: expect 405", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/data", nil)
		mux.ServeHTTP(res, req)
		expectTrue(t, res.Code == http.StatusMethodNotAllowed)
	})

	t.Run("POST /data/1: expect 404", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/data/1", nil)
		mux.ServeHTTP(res, req)
		expectTrue(t, res.Code == http.StatusNotFound)
	})
}

func TestServeMux_RouteWithPathParams(t *testing.T) {
	var visited bool
	mux := NewServeMux()
	mux.Route(Route{
		Method: "GET",
		Path:   "/data/:id",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			id := PathParams(r).ByName("id")
			expectTrue(t, id == "123")
			visited = true
			return nil
		},
	})

	req := httptest.NewRequest("GET", "/data/123", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	expectTrue(t, visited)
}

func TestServeMux_RouteWithMiddleware(t *testing.T) {
	mid := MuxMiddleware(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Trace", "mid-started")
			defer w.Header().Add("X-Trace", "mid-ended")
			return next.ServeHTTP(w, r)
		})
	})

	mux := NewServeMux(Opts.Middleware(mid))

	mux.Route(Route{
		Method: "POST",
		Path:   "/data",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Trace", "POST /data")
			w.WriteHeader(201)
			return nil
		},
	})

	mux.Route(Route{
		Method: "GET",
		Path:   "/data",
		Handler: func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Trace", "GET /data")
			w.WriteHeader(200)
			return nil
		},
	})

	t.Run("POST /data: expect 201", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/data", nil)
		mux.ServeHTTP(res, req)
		traces := strings.Join(res.Header().Values("X-Trace"), ",")
		expectTrue(t, res.Code == 201)
		expectTrue(t, traces == "mid-started,POST /data,mid-ended")
	})

	t.Run("GET /data: expect 200", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/data", nil)
		mux.ServeHTTP(res, req)
		traces := strings.Join(res.Header().Values("X-Trace"), ",")
		expectTrue(t, res.Code == 200)
		expectTrue(t, traces == "mid-started,GET /data,mid-ended")
	})
}

func TestServeMux_RouteWithLastResortError(t *testing.T) {
	anError := errors.New("an error")

	mux := NewServeMux(Opts.LastResortErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, anError) {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, "ErrorResolved")
		} else {
			DefaultHandler.LastResortError(w, r, err)
		}
	}))

	mux.Route(Route{
		Method:  "POST",
		Path:    "/data",
		Handler: func(w http.ResponseWriter, r *http.Request) error { return anError },
	})

	req := httptest.NewRequest("POST", "/data", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	expectTrue(t, res.Code == 400)
	expectTrue(t, strings.TrimSpace(res.Body.String()) == "ErrorResolved")
}

func TestServeMux_RouteWithRouteSpecificMiddleware(t *testing.T) {
	mid := MuxMiddleware(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Trace", "global-mid-started")
			defer w.Header().Add("X-Trace", "global-mid-ended")
			return next.ServeHTTP(w, r)
		})
	})

	local := MuxMiddleware(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Trace", "local-mid-started")
			defer w.Header().Add("X-Trace", "local-mid-ended")
			return next.ServeHTTP(w, r)
		})
	})

	mux := NewServeMux(Opts.Middleware(mid))
	mux.Route(
		Route{Method: "POST", Path: "/data", Handler: func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Trace", "POST /data")
			w.WriteHeader(201)
			return nil
		}},
		local,
	)

	mux.Route(Route{Method: "GET", Path: "/data", Handler: func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Add("X-Trace", "GET /data")
		w.WriteHeader(200)
		return nil
	}})

	t.Run("POST /data: expect 201", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/data", nil)
		mux.ServeHTTP(res, req)
		traces := strings.Join(res.Header().Values("X-Trace"), ",")
		expectTrue(t, res.Code == 201)
		expectTrue(t, traces == "global-mid-started,local-mid-started,POST /data,local-mid-ended,global-mid-ended")
	})

	t.Run("GET /data: expect 200", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/data", nil)
		mux.ServeHTTP(res, req)
		traces := strings.Join(res.Header().Values("X-Trace"), ",")
		expectTrue(t, res.Code == 200)
		expectTrue(t, traces == "global-mid-started,GET /data,global-mid-ended")
	})
}

func expectTrue(t *testing.T, actual bool) {
	t.Helper()
	if !actual {
		t.Fatalf("expected true, got false")
	}
}

func expectFalse(t *testing.T, actual bool) {
	t.Helper()
	if actual {
		t.Fatalf("expected false, got true")
	}
}
