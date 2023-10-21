package httpkit

import (
	"errors"
	"testing"
)

func TestResolvedError_Error(t *testing.T) {
	rootErr := errors.New("an error")
	err := ResolveError(rootErr)

	var resolvedErr *ResolvedError
	expectTrue(t, errors.As(err, &resolvedErr))
	expectTrue(t, errors.Is(resolvedErr.Err, rootErr))
	expectTrue(t, resolvedErr.Error() == rootErr.Error())
}
