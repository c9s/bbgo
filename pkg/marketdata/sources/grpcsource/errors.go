package grpcsource

import (
	"errors"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// isStreamEnd reports whether err means the server finished sending, as opposed
// to something going wrong.
//
// A clean end arrives as io.EOF. A cancelled context surfaces as a gRPC
// Canceled status, which is also not a failure: the consumer closed the cursor.
func isStreamEnd(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}

	switch status.Code(err) {
	case codes.OK, codes.Canceled:
		return true
	default:
		return false
	}
}
