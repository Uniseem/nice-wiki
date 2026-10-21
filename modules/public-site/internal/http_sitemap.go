package publicsite

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"nice-wiki/libs/platform"
	"sort"
	"strings"
	"time"
)

type HandleSitemapIndexInput struct{ DataEpoch string }
type HandleSitemapIndexResult struct {
	Status int
	Xml    string
}
type HandleSitemapShardInput struct {
	Locale    string
	Shard     int
	DataEpoch string
}
type HandleSitemapShardResult struct {
	Status int
	Xml    string
}

// HandleSitemapIndex 按当前权威公开集合生成每语言固定分片地址。
func HandleSitemapIndex(ctx context.Context, op platform.OperationContext, in HandleSitemapIndexInput) (HandleSitemapIndexResult, error) {
	// 站点关闭“允许搜索引擎收录”时不提供这份面向爬虫的文件。
	if enabled, policyErr := readIndexingPolicy(ctx); policyErr != nil {
		return HandleSitemapIndexResult{}, policyErr
	} else if !enabled {
		return HandleSitemapIndexResult{Status: 404}, nil
	}
	directory, err := directoryFrom(ctx)
	if err != nil {
		return HandleSitemapIndexResult{}, err
	}
	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return HandleSitemapIndexResult{}, err
	}
	languages, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil {
		return HandleSitemapIndexResult{}, err
	}
	tags := make([]string, 0, len(languages.versions))
	for tag := range languages.versions {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	locations := make([]string, 0)
	for _, tag := range tags {
		entries, err := BuildSitemapEntries(ctx, op, BuildSitemapEntriesInput{tag, string(in.DataEpoch)})
		if err != nil {
			return HandleSitemapIndexResult{}, err
		}
		for shard := 0; shard*10000 < len(entries.Entries); shard++ {
			locations = append(locations, strings.TrimRight(site.publicBaseURL, "/")+"/sitemap/"+url.PathEscape(tag)+fmt.Sprintf("/%d.xml", shard))
		}
	}
	if err = confirmWatermarks(ctx, directory, site, languages); err != nil {
		return HandleSitemapIndexResult{}, err
	}
	return HandleSitemapIndexResult{Status: 200, Xml: sitemapIndexXML(locations)}, nil
}

// HandleSitemapShard 越界不回退其他分片，所有条目在 BuildSitemapEntries 中最终重验。
func HandleSitemapShard(ctx context.Context, op platform.OperationContext, in HandleSitemapShardInput) (HandleSitemapShardResult, error) {
	// 站点关闭“允许搜索引擎收录”时不提供这份面向爬虫的文件。
	if enabled, policyErr := readIndexingPolicy(ctx); policyErr != nil {
		return HandleSitemapShardResult{}, policyErr
	} else if !enabled {
		return HandleSitemapShardResult{Status: 404}, nil
	}
	if in.Shard < 0 {
		return HandleSitemapShardResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_SHARD")
	}
	entries, err := BuildSitemapEntries(ctx, op, BuildSitemapEntriesInput{in.Locale, string(in.DataEpoch)})
	if err != nil {
		return HandleSitemapShardResult{}, err
	}
	if in.Shard >= len(entries.Entries)/10000+1 || in.Shard*10000 >= len(entries.Entries) {
		return HandleSitemapShardResult{Status: 404, Xml: ""}, nil
	}
	start := in.Shard * 10000
	end := start + 10000
	if end > len(entries.Entries) {
		end = len(entries.Entries)
	}
	return HandleSitemapShardResult{Status: 200, Xml: sitemapURLSetXML(entries.Entries[start:end])}, nil
}

func xmlText(value string) string {
	var output strings.Builder
	_ = xml.EscapeText(&output, []byte(value))
	return output.String()
}
func sitemapIndexXML(locations []string) string {
	var output strings.Builder
	output.WriteString(xml.Header + "<sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	for _, location := range locations {
		output.WriteString("<sitemap><loc>" + xmlText(location) + "</loc></sitemap>")
	}
	output.WriteString("</sitemapindex>")
	return output.String()
}
func sitemapURLSetXML(entries []SitemapEntry) string {
	var output strings.Builder
	output.WriteString(xml.Header + "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	for _, entry := range entries {
		output.WriteString("<url><loc>" + xmlText(entry.Loc) + "</loc><lastmod>" + entry.Lastmod.UTC().Format(time.RFC3339Nano) + "</lastmod></url>")
	}
	output.WriteString("</urlset>")
	return output.String()
}
