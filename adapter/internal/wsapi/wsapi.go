package wsapi

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StatusError reports a non-success response from the REST management API.
type StatusError struct {
	StatusCode   int
	Operation    string
	ConnectionID string
}

func (e *StatusError) Error() string {
	return e.Operation + " status " + httpStatusText(e.StatusCode) + " for " + e.ConnectionID
}

func httpStatusText(statusCode int) string {
	return fmt.Sprintf("%d", statusCode)
}

// IsSafeToRetry reports whether the server explicitly rejected the frame before
// delivery. Transport errors, deadlines, and 5xx responses are ambiguous and
// must not be retried because doing so could duplicate bytes in the stream.
func IsSafeToRetry(err error) bool {
	if err == nil {
		return false
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == 429
	}
	return status.Code(err) == codes.ResourceExhausted
}

// Client is the interface for sending data to WebSocket clients.
type Client interface {
	Send(connectionId string, data []byte, dataType string, iamToken string) error
	Disconnect(connectionId string, iamToken string) error
}

// NewClient creates a REST or gRPC client based on mode.
func NewClient(mode string) Client {
	if mode == "grpc" {
		return &grpcClient{}
	}
	return &restClient{}
}
