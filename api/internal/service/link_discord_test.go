package service

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

// The public link call took both ids from the request with no proof of either; it must stay off.
func TestLinkDiscordAccountIsRefused(t *testing.T) {
	s := &HubServer{}
	_, err := s.LinkDiscordAccount(context.Background(), connect.NewRequest(&hubv1.LinkDiscordAccountRequest{
		UserExternalId: "someone", DiscordUserId: "admin", Username: "admin",
	}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("want Unimplemented, got %v", err)
	}
}
