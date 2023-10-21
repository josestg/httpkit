package httpkit

// ResolvedError is an error that has been resolved.
// When an error is resolved, it means that the error has been mapped to an HTTP response and the no error will be
// handled by the LastResortErrorHandler.
type ResolvedError struct {
	Err error // the original error.
}

// ResolveError marks the error as resolved.
func ResolveError(err error) error {
	return &ResolvedError{Err: err}
}

// Error implements error interface.
func (e *ResolvedError) Error() string { return e.Err.Error() }
