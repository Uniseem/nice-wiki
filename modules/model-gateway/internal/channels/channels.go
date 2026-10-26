// Package channels 管理用户自带的 AI 渠道（BYOK）：OpenAI 兼容地址 + key + 模型名。
// 渠道落成普通的 providers/models/credentials 行（id 以 byok- 开头），路由时排在平台渠道前面；
// key 用与平台渠道相同的主密钥加密保存，保存后不再回显，只留后四位。
package channels

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/netguard"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/ownership"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/security"
)

// Prefix 是用户渠道的供应方标识前缀。
const Prefix = ownership.Prefix

const maxChannelsPerUser = 10

type OperationContext = platform.OperationContext

func userOf(op OperationContext) (string, error) {
	if op.Actor.Kind != "admin" || !strings.HasPrefix(string(op.Actor.SiteID), "home:") {
		return "", platform.NewError(platform.CodeForbidden, false, "AI 渠道只能由用户本人在个人设置里管理")
	}
	return strings.TrimPrefix(string(op.Actor.SiteID), "home:"), nil
}

// adminOf 是网关内部写供应方、凭据、模型时使用的管理身份；调用前已经确认是用户本人。
func adminOf(op OperationContext) OperationContext {
	op.Actor.Grants = append(slices.Clone(op.Actor.Grants), "superadmin")
	return op
}

func view(row channelRow) gen.AiChannel {
	var probe gen.ChannelProbe
	_ = json.Unmarshal([]byte(row.probe), &probe)
	var tasks, capabilities []string
	_ = json.Unmarshal([]byte(row.tasks), &tasks)
	_ = json.Unmarshal([]byte(row.capabilities), &capabilities)
	if tasks == nil {
		tasks = []string{}
	}
	created, _ := platform.ParseTimestamp(row.created)
	return gen.AiChannel{ChannelId: row.id, Name: row.name, BaseUrl: row.baseURL, Model: row.modelName, Tasks: tasks, Capabilities: capabilities, KeyLast4: row.last4, CreatedAt: created, Probe: probe}
}

type channelRow struct{ id, name, modelID, modelName, baseURL, last4, tasks, capabilities, probe, created string }

const selectChannel = `SELECT channel_id,name,model_id,model_name,base_url,key_last4,tasks_json,capabilities_json,probe_json,created_at FROM user_channels`

func scan(rows interface{ Scan(...any) error }) (channelRow, error) {
	var r channelRow
	err := rows.Scan(&r.id, &r.name, &r.modelID, &r.modelName, &r.baseURL, &r.last4, &r.tasks, &r.capabilities, &r.probe, &r.created)
	return r, err
}

func ListChannels(ctx context.Context, op OperationContext, _ struct{}) (gen.ListAiChannelsResult, error) {
	userID, err := userOf(op)
	if err != nil {
		return gen.ListAiChannelsResult{}, err
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return gen.ListAiChannelsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	rows, err := store.DB().QueryContext(ctx, selectChannel+` WHERE user_id=? ORDER BY position, created_at`, userID)
	if err != nil {
		return gen.ListAiChannelsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
	}
	defer rows.Close()
	result := gen.ListAiChannelsResult{Channels: []gen.AiChannel{}}
	for rows.Next() {
		row, err := scan(rows)
		if err != nil {
			return result, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
		}
		result.Channels = append(result.Channels, view(row))
	}
	return result, nil
}

// probe 向渠道发一次最小的对话请求验证地址、key 与模型名；再试一次带工具定义的请求，看是否支持工具调用。
// 只返回结论与简短原因，不回显 key 和供应方返回的正文。
func probe(ctx context.Context, baseURL, apiKey, model string) gen.ChannelProbe {
	started := time.Now()
	result := gen.ChannelProbe{}
	client := &http.Client{
		Timeout:       25 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 8 * time.Second, Control: netguard.Control(catalog.AllowLoopbackHTTP)}).DialContext,
			TLSHandshakeTimeout: 8 * time.Second,
		},
	}
	send := func(body map[string]any) (int, []byte, error) {
		raw, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(raw))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return response.StatusCode, payload, nil
	}
	status, _, err := send(map[string]any{"model": model, "max_tokens": 8, "messages": []map[string]string{{"role": "user", "content": "ping"}}})
	result.LatencyMs = int(time.Since(started).Milliseconds())
	switch {
	case err != nil:
		result.Message = "连接失败：地址不可达或被拒绝"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		result.Message = "key 无效或没有权限"
	case status == http.StatusPaymentRequired || status == http.StatusTooManyRequests:
		result.Message = "key 余额不足或被限流"
	case status == http.StatusNotFound:
		result.Message = "地址或模型名不对（接口返回 404）"
	case status != http.StatusOK:
		result.Message = "渠道返回了错误状态"
	default:
		result.Ok, result.Chat = true, true
		toolStatus, _, toolErr := send(map[string]any{"model": model, "max_tokens": 16, "messages": []map[string]string{{"role": "user", "content": "ping"}},
			"tools": []map[string]any{{"type": "function", "function": map[string]any{"name": "ping", "description": "ping", "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}}}})
		result.Tools = toolErr == nil && toolStatus == http.StatusOK
		result.Message = "可用"
	}
	return result
}

func randomHex(n int) string {
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

// CreateChannel 校验地址 → 探测 → 加密保存 key → 登记渠道；探测不通过就不保存。
func CreateChannel(ctx context.Context, op OperationContext, input gen.CreateAiChannelInput) (gen.AiChannelResult, error) {
	userID, err := userOf(op)
	if err != nil {
		return gen.AiChannelResult{}, err
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	var existing int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_channels WHERE user_id=?`, userID).Scan(&existing); err != nil {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
	}
	if existing >= maxChannelsPerUser {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "自带渠道数量已达上限")
	}
	admin := adminOf(op)
	validated, err := catalog.ValidateProvider(ctx, admin, catalog.ValidateProviderInput{Provider: gen.Provider{ProviderId: platform.NewIdentifier(), Version: 1, Adapter: "openai_chat", BaseUrl: input.BaseUrl,
		Enabled: true, ConnectTimeoutMs: 8000, RequestTimeoutMs: 120000, DisplayName: input.Name, ExtraHeaderNames: []string{}, ExtraHeaderSecretRefs: []string{}, ConcurrencyLimit: 4, StreamIdleTimeoutMs: 60000}})
	if err != nil {
		return gen.AiChannelResult{}, err
	}
	parsed, err := url.Parse(validated.NormalizedBaseURL)
	if err != nil {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeValidation, false, "渠道地址不合法")
	}
	if err := netguard.CheckHost(ctx, parsed.Hostname(), catalog.AllowLoopbackHTTP); err != nil {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeValidation, false, "渠道地址不允许指向内网或本机")
	}
	probeResult := probe(ctx, validated.NormalizedBaseURL, input.ApiKey, input.Model)
	if !probeResult.Ok {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeValidation, false, "渠道探测未通过：%s", probeResult.Message)
	}

	providerID := Prefix + firstN(strings.ReplaceAll(userID, "-", ""), 8) + "-" + randomHex(4)
	modelID := providerID + "-m"
	maxInput, maxOutput := 32768, 4096
	if input.MaxInputTokens != nil {
		maxInput = *input.MaxInputTokens
	}
	if input.MaxOutputTokens != nil {
		maxOutput = *input.MaxOutputTokens
	}
	capabilities := []string{"chat", "structured", "translation"}
	if input.Vision != nil && *input.Vision {
		capabilities = append(capabilities, "vision")
	}
	key := randomHex(16)
	if _, err := catalog.PutProvider(ctx, admin, catalog.PutProviderInput{ExpectedVersion: 0, IdempotencyKey: key, Provider: gen.Provider{ProviderId: providerID, Version: 1, Adapter: "openai_chat",
		BaseUrl: validated.NormalizedBaseURL, Enabled: true, CredentialVersion: 0, ConnectTimeoutMs: 8000, RequestTimeoutMs: 120000, DisplayName: input.Name,
		ExtraHeaderNames: []string{}, ExtraHeaderSecretRefs: []string{}, ConcurrencyLimit: 4, StreamIdleTimeoutMs: 60000}}); err != nil {
		return gen.AiChannelResult{}, err
	}
	if _, err := security.PutCredential(ctx, admin, security.PutCredentialInput{ProviderId: providerID, Secret: input.ApiKey, ExpectedVersion: 0, IdempotencyKey: randomHex(16)}); err != nil {
		return gen.AiChannelResult{}, err
	}
	if _, err := catalog.PutModel(ctx, admin, catalog.PutModelInput{ExpectedVersion: 0, IdempotencyKey: randomHex(16), Model: gen.Model{ModelId: modelID, Version: 1, ProviderId: providerID,
		ProviderModelVersion: input.Model, Capabilities: capabilities, Dimension: 0, PreprocessorVersion: "none", MaxInputTokens: maxInput, MaxOutputTokens: maxOutput,
		CatalogOrigin: "manual", ManualOverride: true, Enabled: true}}); err != nil {
		return gen.AiChannelResult{}, err
	}
	tasks := []string{}
	if input.Tasks != nil {
		tasks = *input.Tasks
	}
	tasksJSON, _ := json.Marshal(tasks)
	capabilitiesJSON, _ := json.Marshal(capabilities)
	probeJSON, _ := json.Marshal(probeResult)
	last4 := input.ApiKey
	if len(last4) > 4 {
		last4 = last4[len(last4)-4:]
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO user_channels(channel_id,user_id,name,model_id,model_name,base_url,key_last4,tasks_json,capabilities_json,position,probe_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		providerID, userID, input.Name, modelID, input.Model, validated.NormalizedBaseURL, last4, string(tasksJSON), string(capabilitiesJSON), existing, string(probeJSON), platform.Now()); err != nil {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "保存 AI 渠道失败")
	}
	row, err := scan(store.DB().QueryRowContext(ctx, selectChannel+` WHERE channel_id=?`, providerID))
	if err != nil {
		return gen.AiChannelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
	}
	return gen.AiChannelResult{Channel: view(row)}, nil
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type ChannelRef struct{ ChannelID string }

// DeleteChannel 停用渠道并销毁 key 密文（仍被在途调用引用的历史密文由网关的退役流程回收）。
func DeleteChannel(ctx context.Context, op OperationContext, input ChannelRef) (gen.RemovedResult, error) {
	userID, err := userOf(op)
	if err != nil {
		return gen.RemovedResult{}, err
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
	}
	defer tx.Rollback()
	var owner string
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM user_channels WHERE channel_id=?`, input.ChannelID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != userID) {
		return gen.RemovedResult{}, platform.NewError(platform.CodeNotFound, false, "AI 渠道不存在")
	}
	if err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取 AI 渠道失败")
	}
	now := platform.Now()
	for _, statement := range []string{
		`UPDATE models SET enabled=0, version=version+1, updated_at=? WHERE provider_id=?`,
		`UPDATE providers SET enabled=0, version=version+1, updated_at=? WHERE provider_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, now, input.ChannelID); err != nil {
			return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "停用 AI 渠道失败")
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_channels WHERE channel_id=?`, input.ChannelID); err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "删除 AI 渠道失败")
	}
	// 没有任何调用引用时，直接销毁密文。
	if _, err := tx.ExecContext(ctx, `DELETE FROM credentials WHERE provider_id=? AND NOT EXISTS (SELECT 1 FROM credential_references r WHERE r.provider_id=? AND r.released=0) AND NOT EXISTS (SELECT 1 FROM route_credential_refs r WHERE r.provider_id=? AND r.released=0)`, input.ChannelID, input.ChannelID, input.ChannelID); err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "销毁密文失败")
	}
	if err := tx.Commit(); err != nil {
		return gen.RemovedResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "删除 AI 渠道失败")
	}
	return gen.RemovedResult{Removed: true}, nil
}

// Usage 返回站点所有者的平台额度使用情况（账本里付费方是平台渠道的部分）。
func Usage(ctx context.Context, op OperationContext, _ struct{}) (gen.AiUsageResult, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return gen.AiUsageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库尚未装配")
	}
	policy, err := ownership.Lookup(ctx, string(op.Actor.SiteID), op.DataEpoch)
	if err != nil {
		return gen.AiUsageResult{}, err
	}
	usage := gen.AiUsage{PlatformAllowed: policy.PlatformAllowed, PlatformOwner: policy.PlatformOwner, CapMicros: int(policy.CapMicros)}
	if policy.PeriodStartNs > 0 {
		usage.PeriodStart = time.Unix(0, policy.PeriodStartNs).UTC()
	}
	if len(policy.SiteIDs) == 0 {
		return gen.AiUsageResult{Usage: usage}, nil
	}
	holders := strings.TrimSuffix(strings.Repeat("?,", len(policy.SiteIDs)), ",")
	args := []any{op.DataEpoch, policy.PeriodStartNs}
	for _, id := range policy.SiteIDs {
		args = append(args, id)
	}
	rows, err := store.DB().QueryContext(ctx, `SELECT state,amount_micro_usd FROM fee_commitments WHERE payer='platform' AND data_epoch=? AND (state='held' OR (state='settled' AND created_ns>=?)) AND site_id IN (`+holders+`)`, args...)
	if err != nil {
		return gen.AiUsageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取额度账本失败")
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var amount sql.NullInt64
		if rows.Scan(&state, &amount) != nil {
			continue
		}
		switch {
		case !amount.Valid:
			usage.UnknownCount++
		case state == "settled":
			usage.SpentMicros += int(amount.Int64)
		default:
			usage.HeldMicros += int(amount.Int64)
		}
	}
	return gen.AiUsageResult{Usage: usage}, nil
}
