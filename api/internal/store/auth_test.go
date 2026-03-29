package store

import "testing"

func TestNormalizeDiscordDomainToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "raw token", input: "cc314c71a5e9dd72cfca630a75aef2779fd239cc", want: "cc314c71a5e9dd72cfca630a75aef2779fd239cc"},
		{name: "prefixed token", input: "dh=cc314c71a5e9dd72cfca630a75aef2779fd239cc", want: "cc314c71a5e9dd72cfca630a75aef2779fd239cc"},
		{name: "reject spaces", input: "bad token", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeDiscordDomainToken(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeDiscordDomainToken(%q) unexpectedly succeeded", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDiscordDomainToken(%q) returned error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeDiscordDomainToken(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
