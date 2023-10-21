package httpkit

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSON(t *testing.T) {
	var data struct {
		Name string `json:"name"`
	}

	err := ReadJSON(strings.NewReader(`{"name":"John Doe"}`), &data)
	expectTrue(t, err == nil)
	expectTrue(t, data.Name == "John Doe")

	err = ReadJSON(strings.NewReader(`{"name":"John Doe", "age": 20}`), &data)
	expectTrue(t, err != nil) // unknown field "age"
}

func TestWriteJSON(t *testing.T) {
	var data = struct {
		Name string `json:"name"`
	}{
		Name: "John Doe",
	}

	rec := httptest.NewRecorder()
	err := WriteJSON(rec, data, 200)
	expectTrue(t, err == nil)
	expectTrue(t, rec.Code == 200)
	expectTrue(t, rec.Header().Get("Content-Type") == contentTypeApplicationJSONCharsetUTF8)

	body := rec.Body.String()
	expectTrue(t, body == "{\"name\":\"John Doe\"}\n")

}
