package permissions

import (
	"context"
	"testing"
)

func TestAutoApproveRequester_AllowsWithoutRequest(t *testing.T) {
	requester := NewAutoApproveRequester()

	allowed, err := requester.RequestPermission(context.Background(), "bash", "rm -rf tmp", "", true)
	if err != nil {
		t.Fatalf("RequestPermission() error = %v", err)
	}
	if !allowed {
		t.Fatal("expected auto-approved permission")
	}
	if requester.HasPendingRequest() {
		t.Fatal("expected no pending request")
	}

	select {
	case req := <-requester.GetRequestChan():
		t.Fatalf("expected no permission request, got %#v", req)
	default:
	}
}
