package videoembed

import (
	"context"
	"net/url"
	"strings"
)

type Registry struct{ providers []Provider }

func NewRegistry(client HTTPClient) *Registry {
	return &Registry{providers: []Provider{
		NewYouTube(client), NewBilibili(client), NewTikTok(client), NewDouyin(client),
	}}
}

func (r *Registry) Recognize(raw string) (Provider, string, error) {
	if strings.TrimSpace(raw) != raw || len(raw) > 2048 {
		return nil, "", ErrUnsupported
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" {
		return nil, "", ErrUnsupported
	}
	for _, provider := range r.providers {
		if id, ok := provider.Recognize(parsed); ok {
			return provider, id, nil
		}
	}
	return nil, "", ErrUnsupported
}

func (r *Registry) Resolve(ctx context.Context, raw string) (Metadata, error) {
	provider, id, err := r.Recognize(raw)
	if err != nil {
		return Metadata{}, err
	}
	embed, err := provider.EmbedURL(id)
	if err != nil {
		return Metadata{}, err
	}
	value, fetchErr := provider.Fetch(ctx, id)
	value.Provider, value.ProviderID, value.EmbedURL = provider.Name(), id, embed
	return value, fetchErr
}
