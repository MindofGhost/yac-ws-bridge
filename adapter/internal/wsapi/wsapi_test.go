package wsapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsSafeToRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "REST throttle", err: &StatusError{StatusCode: http.StatusTooManyRequests}, want: true},
		{name: "REST server error is ambiguous", err: &StatusError{StatusCode: http.StatusServiceUnavailable}, want: false},
		{name: "deadline is ambiguous", err: context.DeadlineExceeded, want: false},
		{name: "transport error is ambiguous", err: errors.New("connection reset"), want: false},
		{name: "gRPC throttle", err: status.Error(codes.ResourceExhausted, "throttled"), want: true},
		{name: "gRPC unavailable is ambiguous", err: status.Error(codes.Unavailable, "unavailable"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSafeToRetry(tt.err); got != tt.want {
				t.Fatalf("IsSafeToRetry(%v) = %v; want %v", tt.err, got, tt.want)
			}
		})
	}
}
