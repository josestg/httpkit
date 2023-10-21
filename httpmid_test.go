package httpkit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReduceNetMiddleware(t *testing.T) {
	mid := ReduceNetMiddleware(
		fakeNetMiddleware("m1", "{", "}"),
		fakeNetMiddleware("m2", "(", ")"),
		fakeNetMiddleware("m3", "[", "]"),
	)

	mux := mid.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Net-Middleware", "h")
	}))

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	mux.ServeHTTP(res, req)

	traces := "m1{m2(m3[h])}"
	actual := strings.Join(res.Header().Values("X-Net-Middleware"), "")
	expectTrue(t, res.Code == 200)
	expectTrue(t, traces == actual)
}

func TestReduceMuxMiddleware(t *testing.T) {
	mid := ReduceMuxMiddleware(
		fakeMuxMiddleware("m1", "{", "}"),
		fakeMuxMiddleware("m2", "(", ")"),
		fakeMuxMiddleware("m3", "[", "]"),
	)

	mux := mid.Then(HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Add("X-Mux-Middleware", "h")
		return nil
	}))

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	err := mux.ServeHTTP(res, req)
	expectTrue(t, err == nil)
	expectTrue(t, res.Code == 200)

	traces := "m1{m2(m3[h])}"
	actual := strings.Join(res.Header().Values("X-Mux-Middleware"), "")
	expectTrue(t, traces == actual)
}

func fakeNetMiddleware(name, start, end string) NetMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Net-Middleware", name+start)
			defer w.Header().Add("X-Net-Middleware", end)
			next.ServeHTTP(w, r)
		})
	}
}

func fakeMuxMiddleware(name, start, end string) MuxMiddleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Add("X-Mux-Middleware", name+start)
			defer w.Header().Add("X-Mux-Middleware", end)
			return next.ServeHTTP(w, r)
		})
	}
}
