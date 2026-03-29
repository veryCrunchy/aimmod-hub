package httpserver

import "testing"

func TestProfileHandleFromRequestHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		requestHost   string
		subdomainHost string
		wantHandle    string
		wantOK        bool
	}{
		{
			name:          "matches profile subdomain",
			requestHost:   "player.aimmod.app",
			subdomainHost: "aimmod.app",
			wantHandle:    "player",
			wantOK:        true,
		},
		{
			name:          "strips request port",
			requestHost:   "player.aimmod.app:8080",
			subdomainHost: "aimmod.app",
			wantHandle:    "player",
			wantOK:        true,
		},
		{
			name:          "ignores apex host",
			requestHost:   "aimmod.app",
			subdomainHost: "aimmod.app",
			wantHandle:    "",
			wantOK:        false,
		},
		{
			name:          "ignores nested subdomains",
			requestHost:   "one.two.aimmod.app",
			subdomainHost: "aimmod.app",
			wantHandle:    "",
			wantOK:        false,
		},
		{
			name:          "rejects invalid label characters",
			requestHost:   "bad_handle.aimmod.app",
			subdomainHost: "aimmod.app",
			wantHandle:    "",
			wantOK:        false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotHandle, gotOK := profileHandleFromRequestHost(tc.requestHost, tc.subdomainHost)
			if gotHandle != tc.wantHandle || gotOK != tc.wantOK {
				t.Fatalf("profileHandleFromRequestHost(%q, %q) = (%q, %v), want (%q, %v)", tc.requestHost, tc.subdomainHost, gotHandle, gotOK, tc.wantHandle, tc.wantOK)
			}
		})
	}
}
