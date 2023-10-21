package httpkit

import (
	"encoding/json"
	"io"
	"net/http"
)

// ReadJSON reads json from the reader and decodes it to the data.
// By default, it disallows unknown fields.
func ReadJSON(r io.Reader, data any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec.Decode(data)
}

// WriteJSON writes the data to the response writer as JSON.
// By default, it sets the content type to application/json; charset=utf-8.
func WriteJSON(w http.ResponseWriter, data any, code int) error {
	writeContentTypeAndStatus(w, contentTypeApplicationJSONCharsetUTF8, code)
	return json.NewEncoder(w).Encode(data)
}

// writeContentTypeAndStatus writes the content type and status code to the response writer.
func writeContentTypeAndStatus(w http.ResponseWriter, value string, code int) {
	w.Header().Add("Content-Type", value)
	w.WriteHeader(code)
}

// MIMETypes
const (
	charsetUTF8                           = "charset=UTF-8"
	contextTypeApplicationJSON            = "application/json"
	contentTypeApplicationJSONCharsetUTF8 = contextTypeApplicationJSON + "; " + charsetUTF8
)
