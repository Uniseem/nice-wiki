package publicsite

import (
	"context"
	"fmt"
	"net/url"
	"nice-wiki/libs/platform"
	"strings"
)

type HandleLanguageSitemapInput struct {
	Locale    string
	DataEpoch string
}
type HandleLanguageSitemapResult struct {
	Status int
	Xml    string
}

// HandleLanguageSitemap 每语言不超过一万项时直接返回 urlset，超过时只列该语言分片。
func HandleLanguageSitemap(ctx context.Context, op platform.OperationContext, in HandleLanguageSitemapInput) (HandleLanguageSitemapResult, error) {
	entries, err := BuildSitemapEntries(ctx, op, BuildSitemapEntriesInput{in.Locale, string(in.DataEpoch)})
	if err != nil {
		return HandleLanguageSitemapResult{}, err
	}
	if len(entries.Entries) <= 10000 {
		return HandleLanguageSitemapResult{Status: 200, Xml: sitemapURLSetXML(entries.Entries)}, nil
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return HandleLanguageSitemapResult{}, err
	}
	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return HandleLanguageSitemapResult{}, err
	}
	locations := make([]string, 0)
	for shard := 0; shard*10000 < len(entries.Entries); shard++ {
		locations = append(locations, strings.TrimRight(site.publicBaseURL, "/")+"/sitemap/"+url.PathEscape(in.Locale)+fmt.Sprintf("/%d.xml", shard))
	}
	return HandleLanguageSitemapResult{Status: 200, Xml: sitemapIndexXML(locations)}, nil
}
