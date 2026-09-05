package osu

import (
	"context"
	"fmt"
	"net/url"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type osuckAdapter struct {
	client *upstreamClient
}

func newOsuckAdapter(client *upstreamClient) *osuckAdapter {
	return &osuckAdapter{client: skinUpstreamClient(client)}
}

func (a *osuckAdapter) status(ctx context.Context) *osuv1.SkinProviderStatus {
	status := baseSkinProviderStatus(osuv1.SkinProvider_SKIN_PROVIDER_OSUCK)
	status.ContractIsDocumented = false
	status.Retryable = true
	if _, err := a.client.get(ctx, "/", nil, ""); err != nil {
		status.Message = "skins.osuck.net is currently unavailable to Hub: " + err.Error() + ". Retry later or open the provider in your browser."
		return status
	}
	status.Message = "skins.osuck.net responded, but Hub cannot currently read its catalog format. Open the provider in your browser."
	return status
}

func (a *osuckAdapter) search(ctx context.Context, req *osuv1.SearchSkinsRequest, pageToken string) ([]*osuv1.SkinItem, string, error) {
	if pageToken != "" {
		return nil, "", fmt.Errorf("skins.osuck.net has no active pagination cursor; restart the search")
	}
	if _, err := a.client.get(ctx, "/search", url.Values{"query": {req.GetQuery()}}, ""); err != nil {
		return nil, "", fmt.Errorf("skins.osuck.net search is currently unavailable: %w. Retry later or search on the provider's website", err)
	}
	return nil, "", fmt.Errorf("skins.osuck.net responded, but Hub cannot currently read its search format. Search on the provider's website")
}

func (a *osuckAdapter) detail(ctx context.Context, sourceID string) (*osuv1.SkinItem, error) {
	if skinPageURL(osuv1.SkinProvider_SKIN_PROVIDER_OSUCK, sourceID) == "" {
		return nil, fmt.Errorf("skins.osuck.net source_id must be a canonical positive integer")
	}
	if _, err := a.client.get(ctx, "/skins/"+sourceID, nil, ""); err != nil {
		return nil, fmt.Errorf("skins.osuck.net detail is currently unavailable: %w", err)
	}
	return nil, fmt.Errorf("skins.osuck.net responded, but Hub cannot currently read its detail format. Open the skin on the provider's website")
}

func (a *osuckAdapter) downloadHandoff(_ context.Context, sourceID string) (*osuv1.SkinDownloadHandoff, error) {
	page := skinPageURL(osuv1.SkinProvider_SKIN_PROVIDER_OSUCK, sourceID)
	if page == "" {
		return nil, fmt.Errorf("skins.osuck.net source_id must be a canonical positive integer")
	}
	return browserSkinHandoff(page+"?tab=downloads", "Choose a download variant on skins.osuck.net. Hub has not verified a direct archive URL."), nil
}
