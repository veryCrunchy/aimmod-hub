package osu

import (
	"context"
	"fmt"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type osuckAdapter struct {
	client *upstreamClient
}

func newOsuckAdapter(client *upstreamClient) *osuckAdapter {
	return &osuckAdapter{client: client}
}

func (a *osuckAdapter) status(ctx context.Context) *osuv1.SkinProviderStatus {
	status := baseSkinProviderStatus(osuv1.SkinProvider_SKIN_PROVIDER_OSUCK)
	status.ContractIsDocumented = false
	if _, err := a.client.get(ctx, "/skins", nil, ""); err != nil {
		status.Message = "skins.osuck.net does not expose a verified server-to-server catalog to AimMod Hub: " + err.Error()
		return status
	}
	status.Message = "skins.osuck.net loaded, but AimMod Hub has not verified a stable server-side search and detail contract."
	return status
}

func (a *osuckAdapter) search(context.Context, *osuv1.SearchSkinsRequest, string) ([]*osuv1.SkinItem, string, error) {
	return nil, "", fmt.Errorf("skins.osuck.net search is unavailable because its public site blocks Hub requests with Cloudflare")
}

func (a *osuckAdapter) detail(context.Context, string) (*osuv1.SkinItem, error) {
	return nil, fmt.Errorf("skins.osuck.net detail is unavailable because its public site blocks Hub requests with Cloudflare")
}

func (a *osuckAdapter) downloadHandoff(context.Context, string) (*osuv1.SkinDownloadHandoff, error) {
	return unavailableSkinHandoff(false, "No skins.osuck.net download handoff is exposed because Hub cannot verify the download redirect and file response."), nil
}
