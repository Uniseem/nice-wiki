package analytics

import (
	"context"
	"nice-wiki/libs/platform"
	"strings"
	"time"
)

// IngestBeacon 只接收可信入口的随机访客标识、地址与设备信息，排入有界内存队列。
func IngestBeacon(ctx context.Context, op OperationContext, in IngestBeaconInput) (IngestBeaconResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return IngestBeaconResult{}, err
	}
	if in.DataEpoch != op.DataEpoch {
		return IngestBeaconResult{}, platform.NewError(platform.CodeVersionMismatch, false, "匿名访问正文代次不符")
	}
	if _, err = ValidateBeacon(ctx, op, ValidateBeaconInput{BeaconID: in.BeaconID, DocumentID: in.DocumentID, Locale: in.Locale, EventType: in.EventType, Status: in.Status, ElapsedMs: in.ElapsedMs, ReferrerDomain: in.ReferrerDomain}); err != nil {
		return IngestBeaconResult{}, err
	}
	rt.mu.Lock()
	minute := time.Now().Unix() / 60
	if rt.rateMinute != minute {
		rt.rateMinute = minute
		rt.rateCount = 0
	}
	if rt.rateCount >= 3000 {
		rt.dropped++
		rt.mu.Unlock()
		return IngestBeaconResult{}, nil
	}
	rt.rateCount++
	rt.mu.Unlock()
	crawler, err := ClassifyCrawler(ctx, op, ClassifyCrawlerInput{UserAgent: in.UserAgent})
	if err != nil {
		return IngestBeaconResult{}, err
	}
	if crawler.KnownCrawler {
		return IngestBeaconResult{}, nil
	}
	if rt.CountingLocation == nil {
		return IngestBeaconResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "可信报表时区未配置")
	}
	now := time.Now().UTC()
	if !platform.ValidIdentifier(in.VisitorID) {
		return IngestBeaconResult{}, platform.NewError(platform.CodeValidation, false, "访客标识不合法")
	}
	peer, err := trustedPeer(in.PeerAddress)
	if err != nil {
		return IngestBeaconResult{}, err
	}
	country, err := LookupCountry(ctx, op, LookupCountryInput{PeerAddress: in.PeerAddress})
	if err != nil {
		return IngestBeaconResult{}, err
	}
	bucket, err := BucketLatency(ctx, op, BucketLatencyInput{ElapsedMs: in.ElapsedMs})
	if err != nil {
		return IngestBeaconResult{}, err
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.queue) >= 4096 {
		rt.dropped++
		return IngestBeaconResult{}, nil
	}
	rt.queue = append(rt.queue, beaconRow{ID: in.BeaconID, DocumentID: in.DocumentID, Locale: in.Locale, EventType: in.EventType, Status: in.Status, Bucket: bucket.Bucket, DayHash: in.VisitorID, VisitorID: in.VisitorID, IP: peer.String(), UserAgent: in.UserAgent, Source: strings.ToLower(in.ReferrerDomain), Country: country.CountryCode, At: now.Format(analyticsTimeLayout), Epoch: rt.Epoch, QueuedAt: now})
	return IngestBeaconResult{Accepted: true}, nil
}
