package publicsite

import (
	"context"
	"net/url"
	"nice-wiki/libs/platform"
	"reflect"
	"sort"
	"time"
)

type SitemapEntry struct {
	Loc        string            `json:"loc"`
	Lastmod    time.Time         `json:"lastmod"`
	Alternates []AlternateLocale `json:"alternates"`
}
type BuildSitemapEntriesInput struct {
	Locale    string
	DataEpoch string
}
type BuildSitemapEntriesResult struct {
	Entries  []SitemapEntry
	Revision int
}

// BuildSitemapEntries 的 lastmod 只取精确发布正文的公开元数据，不用缓存时间。
func BuildSitemapEntries(ctx context.Context, op platform.OperationContext, in BuildSitemapEntriesInput) (BuildSitemapEntriesResult, error) {
	nav, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return BuildSitemapEntriesResult{}, err
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return BuildSitemapEntriesResult{}, err
	}
	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return BuildSitemapEntriesResult{}, err
	}
	base, err := parsePublicBase(site.publicBaseURL)
	if err != nil {
		return BuildSitemapEntriesResult{}, err
	}
	// lastmod 取公开树同一快照里的发布时间；逐篇读正文会让几百篇的站点 sitemap 超时。
	entries := make([]SitemapEntry, 0, len(nav.Tree))
	for _, node := range nav.Tree {
		relative, err := url.Parse(node.CanonicalPath)
		if err != nil {
			return BuildSitemapEntriesResult{}, err
		}
		entries = append(entries, SitemapEntry{base.ResolveReference(relative).String(), node.LastPublishedAt, []AlternateLocale{}})
	}
	again, err := BuildNavigation(ctx, op, BuildNavigationInput{in.Locale, in.DataEpoch, 10000})
	if err != nil {
		return BuildSitemapEntriesResult{}, err
	}
	if again.Revision != nav.Revision || !reflect.DeepEqual(again.Tree, nav.Tree) {
		return BuildSitemapEntriesResult{}, stale()
	}
	if current, err := readSiteSnapshot(ctx, directory.SiteConfig); err != nil {
		return BuildSitemapEntriesResult{}, err
	} else if current != site {
		return BuildSitemapEntriesResult{}, stale()
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Loc < entries[j].Loc })
	return BuildSitemapEntriesResult{entries, nav.Revision}, nil
}
