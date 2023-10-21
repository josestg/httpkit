package httpkit

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/valyala/bytebufferpool"
)

// LogBodyReader knows how to read the request or response body.
type LogBodyReader interface {
	io.ReaderFrom
	Len() int
	Bytes() []byte
}

// LogEntry contains recorded request and response information.
type LogEntry struct {
	StatusCode  int
	RespondedAt int64
	RequestedAt int64

	// DiscardReqBody and DiscardResBody are used to indicate whether the request
	// or response body should be discarded. By default, both are false.
	DiscardReqBody bool
	DiscardResBody bool

	reqBody *bytebufferpool.ByteBuffer
	resBody *bytebufferpool.ByteBuffer
}

// ReqBody returns the request body.
func (l *LogEntry) ReqBody() LogBodyReader { return l.reqBody }

// ResBody returns the response body.
func (l *LogEntry) ResBody() LogBodyReader { return l.resBody }

// LogEntryRecorder is a middleware that records the request and response on demand.
// The request body is not recorded until it is read by the handler. And the response
// body is not recorded until it is written by the handler.
//
// The recorded entry can be retrieved by calling GetLogEntry(w http.ResponseWriter).
func LogEntryRecorder(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := newLogEntryRecorder(w, r)
		defer putLogEntryRecorder(rec)
		next.ServeHTTP(rec, rec.withRequest(r))
	})
}

// GetLogEntry gets the recorded LogEntry from the given http.ResponseWriter by unwrapping
// the http.ResponseWriter, if not found, it returns false.
func GetLogEntry(w http.ResponseWriter) (*LogEntry, bool) {
	rw := w
	for {
		switch t := rw.(type) {
		case *logEntryRecorder:
			return t.log, true
		case interface{ Unwrap() http.ResponseWriter }:
			rw = t.Unwrap()
		default:
			return nil, false
		}
	}
}

var _recorderPool = &sync.Pool{
	New: func() interface{} {
		return &logEntryRecorder{log: &LogEntry{}}
	},
}

func newLogEntryRecorder(w http.ResponseWriter, r *http.Request) *logEntryRecorder {
	rec := _recorderPool.Get().(*logEntryRecorder)
	rec.req = r.Body
	rec.ResponseWriter = w
	rec.log.StatusCode = 0
	rec.log.RespondedAt = 0
	rec.log.DiscardReqBody = false
	rec.log.DiscardResBody = false
	rec.log.reqBody = bytebufferpool.Get()
	rec.log.resBody = bytebufferpool.Get()
	rec.log.RequestedAt = time.Now().UnixNano()
	return rec
}

func putLogEntryRecorder(rec *logEntryRecorder) {
	if rec.log != nil {
		if rec.log.reqBody != nil {
			bytebufferpool.Put(rec.log.reqBody)
		}
		if rec.log.resBody != nil {
			bytebufferpool.Put(rec.log.resBody)
		}
	}
	rec.req = nil
	rec.ResponseWriter = nil
	_recorderPool.Put(rec)
}

type logEntryRecorder struct {
	http.ResponseWriter
	req io.ReadCloser
	log *LogEntry
}

func (l *logEntryRecorder) Read(p []byte) (n int, err error) {
	n, err = l.req.Read(p)
	if !l.log.DiscardReqBody && n > 0 {
		n, err = l.log.reqBody.Write(p[:n])
	}
	return
}

func (l *logEntryRecorder) Close() (err error) {
	if l.req != nil {
		// propagate the close to the original request body.
		err = l.req.Close()
	}
	return err
}

func (l *logEntryRecorder) withRequest(r *http.Request) *http.Request {
	// register the request body reader for future use.
	l.req = r.Body
	// replace the request body with the recorder, so when the handler reads
	// the recording will be started.
	r.Body = l
	return r
}

func (l *logEntryRecorder) WriteHeader(code int) {
	if l.log.RespondedAt > 0 {
		// if already committed. ignore the write header.
		return
	}

	// delegate to the original ResponseWriter.
	l.ResponseWriter.WriteHeader(code)
	l.log.StatusCode = code
	l.log.RespondedAt = time.Now().UnixNano()
}

func (l *logEntryRecorder) Write(b []byte) (int, error) {
	if l.log.RespondedAt <= 0 {
		// if not committed yet, commit it with http.StatusOK as default.
		l.WriteHeader(http.StatusOK)
	}

	n, err := l.ResponseWriter.Write(b)
	if !l.log.DiscardReqBody && err == nil {
		n, err = l.log.resBody.Write(b[:n])
	}
	return n, err
}

func (l *logEntryRecorder) Unwrap() http.ResponseWriter { return l.ResponseWriter }
