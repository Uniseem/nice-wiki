package publicsite

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// QAProbe 询问 qa 当前是否对访客开放问答；结果缓存 30 秒，qa 不可达时按"未开放"处理，不影响页面。
type QAProbe struct {
	base   string
	client *http.Client
	mu     sync.Mutex
	probes map[string]qaProbeResult // 按站点缓存
}

type qaProbeResult struct {
	at time.Time
	ok bool
}

func NewQAProbe(base, issuer string, key platform.SigningKey) *QAProbe {
	if base == "" {
		return nil
	}
	return &QAProbe{probes: map[string]qaProbeResult{}, base: strings.TrimRight(base, "/"), client: &http.Client{Transport: serviceauth.SigningTransport(issuer, "qa", key, nil), Timeout: 3 * time.Second}}
}

func (p *QAProbe) Available(ctx context.Context, op platform.OperationContext) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	site := string(op.Actor.SiteID)
	if cached, ok := p.probes[site]; ok && time.Since(cached.at) < 30*time.Second {
		return cached.ok
	}
	p.probes[site] = qaProbeResult{at: time.Now()}
	deadline := time.Now().UTC().Add(3 * time.Second)
	id := platform.NewIdentifier()
	probeOp := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: "public-site", Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: op.DataEpoch,
		Actor: platform.AuthContext{Kind: "service", SubjectID: "public-site", SiteID: op.Actor.SiteID, ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
	work, cancel := context.WithDeadline(platform.WithPortalProof(ctx, ""), deadline)
	defer cancel()
	request, err := http.NewRequestWithContext(platform.WithOperationContext(work, probeOp), http.MethodGet, p.base+"/v1/public-capabilities", nil)
	if err != nil {
		return false
	}
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
	var result struct {
		Available bool `json:"available"`
	}
	available := response.StatusCode == http.StatusOK && json.Unmarshal(raw, &result) == nil && result.Available
	p.probes[site] = qaProbeResult{at: p.probes[site].at, ok: available}
	return available
}
