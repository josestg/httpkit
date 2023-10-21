package httpkit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLogEntryRecorder(t *testing.T) {
	raw := `{"foo":"bar"}`

	mux := http.NewServeMux()

	var visited bool
	mux.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		defer func() { visited = true }()
		rec, ok := GetLogEntry(w)
		expectTrue(t, ok)
		expectTrue(t, rec != nil)

		// before reads from request body and writes to response writer.
		expectTrue(t, rec.StatusCode == 0)
		expectTrue(t, rec.RespondedAt == 0)
		expectTrue(t, rec.RequestedAt != 0)
		reqBody := rec.ReqBody()
		resBody := rec.ResBody()
		expectTrue(t, reqBody.Len() == 0)
		expectTrue(t, resBody.Len() == 0)
		expectFalse(t, rec.DiscardReqBody)
		expectFalse(t, rec.DiscardResBody)

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// after reads from request body.
		expectTrue(t, rec.StatusCode == 0)
		expectTrue(t, rec.RespondedAt == 0)
		expectTrue(t, rec.RequestedAt != 0)
		expectTrue(t, reqBody.Len() != 0)
		expectTrue(t, resBody.Len() == 0)
		expectFalse(t, rec.DiscardReqBody)
		expectFalse(t, rec.DiscardResBody)
		expectTrue(t, bytes.Equal(reqBody.Bytes(), []byte(raw)))

		// write to response writer.
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, raw)

		// after writes to response writer.
		expectTrue(t, rec.StatusCode == http.StatusOK)
		expectTrue(t, rec.RespondedAt != 0)
		expectTrue(t, rec.RequestedAt != 0)
		expectTrue(t, (rec.RespondedAt-rec.RequestedAt) >= int64(200*time.Millisecond))
		expectTrue(t, reqBody.Len() != 0)
		expectTrue(t, resBody.Len() != 0)
		expectFalse(t, rec.DiscardReqBody)
		expectFalse(t, rec.DiscardResBody)
		expectTrue(t, bytes.Equal(resBody.Bytes(), []byte(raw)))
		expectTrue(t, bytes.Equal(reqBody.Bytes(), []byte(raw)))

		// just for testing
		_ = r.Body.Close()
		w.WriteHeader(http.StatusBadRequest)
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/foo", strings.NewReader(raw))
	LogEntryRecorder(mux).ServeHTTP(res, req)

	expectTrue(t, visited)
}

func TestGetLogEntry_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		rec, ok := GetLogEntry(w)
		expectFalse(t, ok)
		expectTrue(t, rec == nil)
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/foo", nil)
	mux.ServeHTTP(res, req)
}

func TestGetLogEntry_Wrapped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		rec, ok := GetLogEntry(w)
		expectTrue(t, ok)
		expectTrue(t, rec != nil)
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/foo", nil)

	mid := ReduceNetMiddleware(LogEntryRecorder, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&rwWrapper{ResponseWriter: w}, r)
		})
	})

	mid.Then(mux).ServeHTTP(res, req)
}

func TestLogEntryRecorder_Unwrap(t *testing.T) {

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/foo", nil)
	mid := LogEntryRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unwrap := w.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
		expectTrue(t, unwrap == res)
	}))

	mid.ServeHTTP(res, req)
}

type rwWrapper struct{ http.ResponseWriter }

func (w *rwWrapper) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func BenchmarkLogEntryRecorder(b *testing.B) {
	var content = []byte(`[{"_id":"test-date-1","index":0,"guid":"1c850fdd-3aee-48f7-b9ce-3d6781324177","isActive":false,"balance":"$2,672.30","picture":"http://placehold.it/32x32","age":22,"eyeColor":"green","name":"Benson Macias","gender":"male","company":"CUBICIDE","email":"bensonmacias@cubicide.com","phone":"+1 (930) 487-3458","address":"587 Vernon Avenue, Robinette, New Hampshire, 1059","about":"Sunt ad nostrud quis est quis cupidatat esse do laboris. Sint laborum esse adipisicing irure cillum ipsum cillum excepteur ea. Lorem dolore incididunt Lorem fugiat. Velit amet non quis amet proident non elit dolor culpa ea nulla. Sint ipsum aliqua elit dolor ad aute magna adipisicing.\r\n","registered":"2014-11-16T04:43:23 -07:00","latitude":-22.061117,"longitude":-174.10247,"tags":["mollit","ipsum","culpa","quis","enim","elit","voluptate"],"friends":[{"id":0,"name":"Sonja Sullivan"},{"id":1,"name":"Cook Sutton"},{"id":2,"name":"Donaldson Bruce"}],"greeting":"Hello, Benson Macias! You have 1 unread messages.","favoriteFruit":"apple"},{"_id":"646d671868f792c899294a1a","index":1,"guid":"1aa8918e-77d8-4896-ad7e-fccf97519af7","isActive":false,"balance":"$3,775.77","picture":"http://placehold.it/32x32","age":25,"eyeColor":"brown","name":"Sweeney Peterson","gender":"male","company":"SLUMBERIA","email":"sweeneypeterson@slumberia.com","phone":"+1 (930) 465-2339","address":"758 Gem Street, Nogal, Tennessee, 5108","about":"Id duis officia non voluptate. Laboris qui dolor occaecat amet ipsum fugiat cupidatat do voluptate. Amet consectetur elit mollit laboris dolore exercitation elit nostrud. Irure est adipisicing Lorem ex laborum esse consectetur laborum eu labore et non aliqua esse. Cillum occaecat magna cillum excepteur minim dolore qui laboris ipsum non tempor. Do officia tempor aliqua ex.\r\n","registered":"2015-07-28T08:26:59 -07:00","latitude":80.535895,"longitude":39.756357,"tags":["in","commodo","ipsum","mollit","quis","ad","cillum"],"friends":[{"id":0,"name":"Shanna Stuart"},{"id":1,"name":"Carla Cline"},{"id":2,"name":"Dena Slater"}],"greeting":"Hello, Sweeney Peterson! You have 9 unread messages.","favoriteFruit":"strawberry"},{"_id":"646d67187a5790a1126df22f","index":2,"guid":"f158a67d-aff4-4359-b626-580894c3e4b8","isActive":false,"balance":"$3,278.23","picture":"http://placehold.it/32x32","age":35,"eyeColor":"blue","name":"Earlene Mays","gender":"female","company":"KLUGGER","email":"earlenemays@klugger.com","phone":"+1 (837) 553-3443","address":"416 Hendrickson Street, Beason, Connecticut, 2535","about":"Ad aute duis duis exercitation magna. Et aliqua mollit incididunt eiusmod duis enim qui mollit cupidatat reprehenderit. In duis duis ex aliquip ut culpa ad excepteur ullamco pariatur id velit ipsum. Elit fugiat laborum commodo ut. Quis aute nisi consectetur ex consequat ad sunt ut dolor qui anim mollit nostrud excepteur. Esse esse ad elit excepteur sint cillum.\r\n","registered":"2016-06-01T12:24:36 -07:00","latitude":-38.403202,"longitude":-114.501481,"tags":["qui","reprehenderit","sunt","in","non","incididunt","nostrud"],"friends":[{"id":0,"name":"Gayle Boone"},{"id":1,"name":"Murray Compton"},{"id":2,"name":"Wiggins Marsh"}],"greeting":"Hello, Earlene Mays! You have 6 unread messages.","favoriteFruit":"banana"},{"_id":"646d671860f25524f86c7033","index":3,"guid":"5060431b-6e4a-4ea1-9490-970a07522d15","isActive":true,"balance":"$2,980.25","picture":"http://placehold.it/32x32","age":32,"eyeColor":"green","name":"Compton Gonzalez","gender":"male","company":"PHEAST","email":"comptongonzalez@pheast.com","phone":"+1 (855) 554-3674","address":"554 Martense Street, Greenfields, Colorado, 6145","about":"Culpa anim nisi cillum elit cillum ea. Fugiat enim nisi aliqua ad dolor. Veniam aute laboris esse velit enim aliquip. Elit dolore eiusmod excepteur duis et proident eu.\r\n","registered":"2016-07-19T09:35:48 -07:00","latitude":-41.13686,"longitude":-122.463135,"tags":["irure","minim","fugiat","ad","cillum","do","eiusmod"],"friends":[{"id":0,"name":"Mayer Rodriguez"},{"id":1,"name":"Teri Carver"},{"id":2,"name":"Powell Daniels"}],"greeting":"Hello, Compton Gonzalez! You have 4 unread messages.","favoriteFruit":"strawberry"},{"_id":"646d67183a40719fc0a9124d","index":4,"guid":"6d5f6d33-57a2-478d-be92-5568f645d660","isActive":true,"balance":"$3,602.03","picture":"http://placehold.it/32x32","age":26,"eyeColor":"brown","name":"Hudson Meadows","gender":"male","company":"ACCUPRINT","email":"hudsonmeadows@accuprint.com","phone":"+1 (883) 438-3179","address":"757 Kenilworth Place, Allensworth, North Carolina, 3339","about":"Lorem duis exercitation voluptate laboris. In occaecat qui magna occaecat consequat. Commodo ut magna sit enim magna exercitation labore. Anim reprehenderit sit sint aliquip occaecat est officia ex incididunt velit eiusmod ad eiusmod.\r\n","registered":"2021-09-28T01:45:16 -07:00","latitude":11.613558,"longitude":-167.407339,"tags":["ipsum","enim","officia","proident","eu","aliqua","anim"],"friends":[{"id":0,"name":"Deirdre Maddox"},{"id":1,"name":"Jones England"},{"id":2,"name":"Moore Hebert"}],"greeting":"Hello, Hudson Meadows! You have 3 unread messages.","favoriteFruit":"strawberry"},{"_id":"646d6718033539a99eecf20a","index":5,"guid":"34b9ade5-4de2-4232-8c7c-b6eec3805a0a","isActive":false,"balance":"$1,398.95","picture":"http://placehold.it/32x32","age":24,"eyeColor":"blue","name":"Vickie Norton","gender":"female","company":"GEEKWAGON","email":"vickienorton@geekwagon.com","phone":"+1 (975) 470-3307","address":"505 Stryker Street, Sisquoc, Indiana, 1137","about":"Id eu non proident ut ipsum sit qui est ad ullamco anim voluptate ex. Occaecat consectetur occaecat ullamco reprehenderit nulla qui pariatur minim in sunt commodo irure est voluptate. Labore ea excepteur quis consectetur Lorem. Amet aliquip sint nisi deserunt dolore duis voluptate dolor labore ad consequat est veniam. Tempor fugiat commodo et sit quis. Cupidatat eu voluptate sit aliqua quis ut anim minim incididunt Lorem enim laboris. Mollit do sunt sit magna consequat est aliqua eiusmod nulla quis.\r\n","registered":"2021-03-10T08:56:06 -07:00","latitude":22.958544,"longitude":-161.373822,"tags":["eu","ipsum","qui","non","mollit","voluptate","pariatur"],"friends":[{"id":0,"name":"Brandi Carroll"},{"id":1,"name":"Mckinney Joseph"},{"id":2,"name":"Annabelle Shelton"}],"greeting":"Hello, Vickie Norton! You have 5 unread messages.","favoriteFruit":"banana"}]`)
	body := bytes.NewReader(content)
	const path = "/test"

	handler := http.NewServeMux()
	handler.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	})

	recorder := LogEntryRecorder(handler)
	r := httptest.NewRequest(http.MethodPost, path, body)
	w := &responseWriter{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder.ServeHTTP(w, r)
	}
}

type responseWriter struct{}

func (r *responseWriter) Header() http.Header         { return nil }
func (r *responseWriter) Write(i []byte) (int, error) { return io.Discard.Write(i) }
func (r *responseWriter) WriteHeader(_ int)           {}
