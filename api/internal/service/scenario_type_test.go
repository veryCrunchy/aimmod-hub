package service

import (
	"context"
	"errors"
	"testing"
)

func TestResolveScenarioTypeFillsMissingTypes(t *testing.T) {
	lookups := 0
	server := &HubServer{scenarioTypes: func(_ context.Context, name string) (string, error) {
		lookups++
		switch name {
		case "Synthetic Tracking":
			return "Tracking", nil
		case "Synthetic Failure":
			return "", errors.New("database unavailable")
		default:
			return "", nil
		}
	}}
	ctx := context.Background()
	if got := server.resolveScenarioType(ctx, "Synthetic Tracking", "StaticClicking"); got != "StaticClicking" || lookups != 0 {
		t.Fatalf("a classified run must keep its type without a lookup: %q (%d lookups)", got, lookups)
	}
	if got := server.resolveScenarioType(ctx, "Synthetic Tracking", ""); got != "Tracking" {
		t.Fatalf("empty type = %q, want Tracking", got)
	}
	if got := server.resolveScenarioType(ctx, "Synthetic Tracking", "Unknown"); got != "Tracking" {
		t.Fatalf("Unknown type = %q, want Tracking", got)
	}
	if got := server.resolveScenarioType(ctx, "Synthetic Failure", ""); got != "" {
		t.Fatalf("failed lookup = %q, want the original type", got)
	}
	if got := server.resolveScenarioType(ctx, "Never Classified", "Unknown"); got != "Unknown" {
		t.Fatalf("unclassified scenario = %q, want Unknown", got)
	}
	if got := (&HubServer{}).resolveScenarioType(ctx, "Synthetic Tracking", ""); got != "" {
		t.Fatalf("no lookup configured = %q", got)
	}
}
