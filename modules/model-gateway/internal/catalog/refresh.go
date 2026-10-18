package catalog

import (
	"context"
 "database/sql"
 "errors"
 "nice-wiki/modules/model-gateway/internal/catalog/bundled"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/provider"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"strings"
	"time"
	"unicode/utf8"
)

type RefreshModelCatalogInput = gen.RefreshModelCatalogInput
type RefreshModelCatalogResult struct{CatalogVersion int `json:"catalogVersion"`;Updated int `json:"updated"`;PreservedOverrides int `json:"preservedOverrides"`;Source string `json:"source,omitempty"`;SourceVersion string `json:"sourceVersion,omitempty"`;Fallback string `json:"fallback,omitempty"`}
type CatalogSource struct{Snapshot CatalogSnapshot;Normalized []byte;Origin,SourceHash string;DownloadedNS int64;Fallback string}

// CatalogModel是公开目录中已核的必要事实；未提供费率/上界仍缺失，不替换成零。
type CatalogModel struct {
	ProviderID       string              `json:"providerId"`
	ModelID          string              `json:"modelId"`
	InputLimit       *int                `json:"inputLimit,omitempty"`
	OutputLimit      *int                `json:"outputLimit,omitempty"`
	Price            billing.ManualPrice `json:"price"`
	InputModalities  []string            `json:"inputModalities"`
	OutputModalities []string            `json:"outputModalities"`
}
type CatalogSnapshot map[string]CatalogModel

// DownloadModelCatalog仅固定公开GET，不携站点内容/服务断言/管理员凭据，不重定向。
func DownloadModelCatalog(ctx context.Context) ([]byte, error) {
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(work, http.MethodGet, "https://models.dev/api.json", nil)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "公开目录请求构造失败")
	}
	request.Header.Set("Accept", "application/json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "固定公开模型目录暂不可达，原目录保留")
	}
	defer response.Body.Close()
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || err != nil || mediaType != "application/json" {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录响应状态或MIME不符合固定协议")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (10<<20)+1))
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "公开目录下载未完成")
	}
	if len(raw) > 10<<20 {
		return nil, platform.NewError(platform.CodeBudgetExceeded, false, "公开目录超过10MiB预算")
	}
	return raw, nil
}

// NormalizeModelCatalog只解释公开API明确字段；上游扩展字段不变成本站模型能力/收费事实。
func NormalizeModelCatalog(raw []byte) (CatalogSnapshot, error) {
	if !utf8.Valid(raw) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录非UTF8")
	}
	type remoteModel struct {
		ID   string `json:"id"`
		Cost *struct {
			Input  json.Number `json:"input"`
			Output json.Number `json:"output"`
		} `json:"cost"`
		Limit struct {
			Context *int `json:"context"
 "database/sql"
 "errors"
 "nice-wiki/modules/model-gateway/internal/catalog/bundled"`
			Input   *int `json:"input"`
			Output  *int `json:"output"`
		} `json:"limit"`
		Modalities struct {
			Input  []string `json:"input"`
			Output []string `json:"output"`
		} `json:"modalities"`
	}
	var source map[string]struct {
		Models map[string]remoteModel `json:"models"`
	}
	if json.Unmarshal(raw, &source) != nil || len(source) == 0 || len(source) > 1000 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录根映射损坏或超界")
	}
	result := make(CatalogSnapshot)
	for providerID, entry := range source {
		if !validModelString(providerID) || strings.ContainsRune(providerID, 0) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录渠道身份非法")
		}
		for modelID, model := range entry.Models {
			if len(result) >= 50000 || !validModelString(modelID) || strings.ContainsRune(modelID, 0) || (model.ID != "" && model.ID != modelID) {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录模型身份不一致或超界")
			}
			normalized := CatalogModel{ProviderID: providerID, ModelID: modelID, InputLimit: model.Limit.Input, OutputLimit: model.Limit.Output, InputModalities: append([]string{}, model.Modalities.Input...), OutputModalities: append([]string{}, model.Modalities.Output...)}
			if normalized.InputLimit == nil {
				normalized.InputLimit = model.Limit.Context
			}
			for _, bound := range []*int{normalized.InputLimit, normalized.OutputLimit} {
				if bound != nil && (*bound < 0 || *bound > 9007199254740991) {
					return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录token窗口越界")
				}
			}
			if len(normalized.InputModalities) > 256 || len(normalized.OutputModalities) > 256 {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录模态集合超界")
			}
			for _, mode := range append(append([]string{}, normalized.InputModalities...), normalized.OutputModalities...) {
				if !validBoundedString(mode, 128) {
					return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录模态字符串非法")
				}
			}
			if model.Cost != nil {
				var err error
				normalized.Price.InputPerMillionCost, err = provider.ProviderReportedCost(model.Cost.Input)
				if err != nil {
					return nil, err
				}
				normalized.Price.OutputPerMillionCost, err = provider.ProviderReportedCost(model.Cost.Output)
				if err != nil {
					return nil, err
				}
			}
			// models.dev的token费率未明确本站每次固定费，保留nil，管理员可明确配置0/真实固定费。
			result[providerID+"\x00"+modelID] = normalized
		}
	}
	if len(result) == 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "公开目录没有有效模型事实")
	}
	return result, nil
}

// ReadCatalogSource保留最近真实源；网络不可用不把旧目录冒充新鲜下载。
func ReadCatalogSource(ctx context.Context,op OperationContext)(CatalogSource,error){
 raw,err:=DownloadModelCatalog(ctx)
 if err==nil{snapshot,e:=NormalizeModelCatalog(raw);if e!=nil{return CatalogSource{},e};normalized,e:=platform.CanonicalJSON(snapshot);if e!=nil{return CatalogSource{},platform.NewError(platform.CodeIntegrityError,false,"公开目录规范事实不能编码")};if len(normalized)>10<<20{return CatalogSource{},platform.NewError(platform.CodeBudgetExceeded,false,"公开目录规范事实超过10MiB")};return CatalogSource{Snapshot:snapshot,Normalized:normalized,Origin:"remote",SourceHash:platform.HashBytes(raw),DownloadedNS:time.Now().UnixNano()},nil}
 var failure *platform.Error;if !errors.As(err,&failure)||failure.Code!=platform.CodeDependencyUnavailable{return CatalogSource{},err}
 tx,e:=gatewayruntime.BeginExecution(ctx,op,false);if e!=nil{return CatalogSource{},e};var source CatalogSource;var normalizedHash string;var size int64
 e=tx.Tx.QueryRowContext(tx.Context,`SELECT source,source_hash,normalized_hash,substr(normalized_json,1,10485761),length(normalized_json),downloaded_ns FROM model_catalog_downloads WHERE data_epoch=? ORDER BY version DESC LIMIT 1`,op.DataEpoch).Scan(&source.Origin,&source.SourceHash,&normalizedHash,&source.Normalized,&size,&source.DownloadedNS);tx.Close()
 if e==nil{if size>10<<20||size<1{return CatalogSource{},platform.NewError(platform.CodeBudgetExceeded,false,"持久模型目录规范事实超过10MiB")};if !platform.ValidHash(source.SourceHash)||source.DownloadedNS<=0||(source.Origin!="remote"&&source.Origin!="bundled")||platform.HashBytes(source.Normalized)!=normalizedHash||json.Unmarshal(source.Normalized,&source.Snapshot)!=nil||len(source.Snapshot)==0{return CatalogSource{},platform.NewError(platform.CodeIntegrityError,false,"最近持久模型目录来源或摘要损坏")};canonical,_:=platform.CanonicalJSON(source.Snapshot);if platform.HashBytes(canonical)!=normalizedHash{return CatalogSource{},platform.NewError(platform.CodeIntegrityError,false,"最近持久模型目录含不可丢弃字段")};source.Fallback="cached";return source,nil}
 if e!=sql.ErrNoRows{return CatalogSource{},gatewayruntime.ExecutionStorageError(e)}
 raw,metadata,e:=bundled.ReadSnapshot();if e!=nil{return CatalogSource{},e};snapshot,e:=NormalizeModelCatalog(raw);if e!=nil{return CatalogSource{},e};normalized,e:=platform.CanonicalJSON(snapshot);if e!=nil{return CatalogSource{},platform.NewError(platform.CodeIntegrityError,false,"编译目录规范事实不能编码")};if len(normalized)>10<<20{return CatalogSource{},platform.NewError(platform.CodeBudgetExceeded,false,"编译目录规范事实超过10MiB")};retrieved,_:=time.Parse(time.RFC3339Nano,metadata.RetrievedAt)
 return CatalogSource{Snapshot:snapshot,Normalized:normalized,Origin:"bundled",SourceHash:metadata.SHA256,DownloadedNS:retrieved.UnixNano(),Fallback:"bundled"},nil
}

func RefreshModelCatalog(ctx context.Context, op OperationContext, input RefreshModelCatalogInput) (RefreshModelCatalogResult, error) {
	background := op.CallerService == "model-gateway" && op.Actor.Kind == "service" && op.Actor.SubjectID == "model-gateway"
	if !background {
		if err := requireSuperadmin(op); err != nil {
			return RefreshModelCatalogResult{}, err
		}
	}
	if input.SourceUrl != "https://models.dev/api.json" {
		return RefreshModelCatalogResult{}, platform.NewError(platform.CodeValidation, false, "公开目录来源必须固定models.dev/api.json")
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return RefreshModelCatalogResult{}, err
	}
	var result RefreshModelCatalogResult
	replay, err := gatewayruntime.ReplayExecutionReceipt(tx, op, "model-gateway.RefreshModelCatalog", input.IdempotencyKey, input, &result)
	tx.Close()
	if err != nil || replay {
		return result, err
	}
 source,err:=ReadCatalogSource(ctx,op)
 if err!=nil{return result,err}
 snapshot,normalized:=source.Snapshot,source.Normalized
	write, err := gatewayruntime.BeginExecution(ctx, op, true)
	if err != nil {
		return result, err
	}
	defer write.Close()
	replay, err = gatewayruntime.ReplayExecutionReceipt(write, op, "model-gateway.RefreshModelCatalog", input.IdempotencyKey, input, &result)
	if err != nil || replay {
		return result, err
	}
	var previousHash,previousOrigin,previousSourceHash string
	var previousVersion int
	rows, err := write.Tx.QueryContext(write.Context, `SELECT version,normalized_hash,source,source_hash FROM model_catalog_downloads WHERE data_epoch=? ORDER BY version DESC LIMIT 1`, op.DataEpoch)
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	if rows.Next() {
		if err := rows.Scan(&previousVersion, &previousHash,&previousOrigin,&previousSourceHash); err != nil {
			rows.Close()
			return result, gatewayruntime.ExecutionStorageError(err)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, gatewayruntime.ExecutionStorageError(err)
	}
	hash := platform.HashBytes(normalized)
 if source.Fallback!=""&&previousVersion>0&&(previousHash!=hash||previousOrigin!=source.Origin||previousSourceHash!=source.SourceHash){return result,platform.NewError(platform.CodeVersionMismatch,true,"降级目录选定后最近源已改变，不能覆盖较新真实源")}
	result.CatalogVersion = previousVersion
	if previousHash != hash||previousOrigin!=source.Origin||previousSourceHash!=source.SourceHash {
		inserted, err := write.Tx.ExecContext(write.Context, `INSERT INTO model_catalog_downloads(data_epoch,source,source_hash,normalized_hash,normalized_json,downloaded_ns) VALUES(?,?,?,?,?,?)`, op.DataEpoch,source.Origin,source.SourceHash,hash,normalized,source.DownloadedNS)
		if err != nil {
			return result, gatewayruntime.ExecutionStorageError(err)
		}
		version, err := inserted.LastInsertId()
		if err != nil || version > 9007199254740991 {
			return result, platform.NewError(platform.CodeIntegrityError, false, "公开目录版本不可表示")
		}
		result.CatalogVersion = int(version)
	}
	updated, preserved, err := ApplyCatalogSnapshot(write, op, snapshot,source.Origin)
	if err != nil {
		return result, err
	}
	result.Source=source.Origin;result.SourceVersion="sha256:"+source.SourceHash;result.Fallback=source.Fallback
	result.Updated = updated
	result.PreservedOverrides = preserved
	if err := gatewayruntime.SaveExecutionReceipt(write, op, "model-gateway.RefreshModelCatalog", input.IdempotencyKey, input, result); err != nil {
		return result, err
	}
	if err := write.Commit(); err != nil {
		return RefreshModelCatalogResult{}, err
	}
	return result, nil
}

func ApplyCatalogSnapshot(tx *gatewayruntime.ExecutionTransaction, op OperationContext, snapshot CatalogSnapshot, origin string) (int, int, error) {
	var oldEpoch bool
	if err := tx.Tx.QueryRowContext(tx.Context, `SELECT EXISTS(SELECT 1 FROM models WHERE data_epoch<>?)`, op.DataEpoch).Scan(&oldEpoch); err != nil {
		return 0, 0, gatewayruntime.ExecutionStorageError(err)
	}
	if oldEpoch {
		return 0, 0, platform.NewError(platform.CodeStaleResult, false, "模型目录含未迁移的旧代身份")
	}
	type current struct {
		ID, Origin                       string
		Version, InputLimit, OutputLimit int
		Override                         bool
		Provider, Model                  string
		BindingHash                      string
		Capabilities                     []byte
	}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT m.model_id,m.version,m.max_input_tokens,m.max_output_tokens,m.manual_override,m.catalog_origin,COALESCE(m.catalog_provider_id,''),COALESCE(m.catalog_model_id,''),COALESCE(b.source_hash,''),m.capabilities_json FROM models m LEFT JOIN model_catalog_bindings b ON b.model_id=m.model_id AND b.data_epoch=m.data_epoch AND b.provider_id=m.catalog_provider_id AND b.source_model_id=m.catalog_model_id ORDER BY m.model_id`)
	if err != nil {
		return 0, 0, gatewayruntime.ExecutionStorageError(err)
	}
	items := make([]current, 0)
	for rows.Next() {
		var item current
		if len(items) >= 1000 {
			rows.Close()
			return 0, 0, platform.NewError(platform.CodeBudgetExceeded, false, "当前模型超过目录更新预算")
		}
		if err := rows.Scan(&item.ID, &item.Version, &item.InputLimit, &item.OutputLimit, &item.Override, &item.Origin, &item.Provider, &item.Model, &item.BindingHash, &item.Capabilities); err != nil {
			rows.Close()
			return 0, 0, gatewayruntime.ExecutionStorageError(err)
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, 0, gatewayruntime.ExecutionStorageError(err)
	}
	updated, preserved := 0, 0
	for _, item := range items {
		if item.Override {
			preserved++
			continue
		}
		if (item.Origin != "remote" && item.Origin != "bundled") || item.Provider == "" || item.Model == "" {
			continue
		}
		remote, exists := snapshot[item.Provider+"\x00"+item.Model]
		if !exists {
			continue
		}
		remoteRaw, _ := platform.CanonicalJSON(remote)
		sourceHash := platform.HashBytes(remoteRaw)
		if item.Version >= 9007199254740991 {
			return 0, 0, platform.NewError(platform.CodeIntegrityError, false, "模型版本达到上限")
		}
		inputLimit, outputLimit := item.InputLimit, item.OutputLimit
		if remote.InputLimit != nil {
			inputLimit = *remote.InputLimit
		}
		if remote.OutputLimit != nil {
			outputLimit = *remote.OutputLimit
		}
		if item.BindingHash == sourceHash && inputLimit == item.InputLimit && outputLimit == item.OutputLimit && item.Origin==origin {
			continue
		}
		var capabilities []string
		if json.Unmarshal(item.Capabilities, &capabilities) != nil {
			return 0, 0, platform.NewError(platform.CodeIntegrityError, false, "原模型能力证据损坏")
		}
		for _, capability := range capabilities {
			if ((capability == "file" || capability == "vision") && inputLimit < 1) || ((capability == "chat" || capability == "translation") && outputLimit < 1) {
				return 0, 0, platform.NewError(platform.CodeIntegrityError, false, "远端上界与模型已声明能力矛盾")
			}
		}
		// 空间/渠道/预处理与管理员声明能力不从attachment或模型名推断、改写。
		changed, err := tx.Tx.ExecContext(tx.Context, `UPDATE models SET version=version+1,max_input_tokens=?,max_output_tokens=?,catalog_origin=?,updated_at=? WHERE model_id=? AND version=? AND data_epoch=? AND manual_override=0 AND catalog_provider_id=? AND catalog_model_id=?`, inputLimit, outputLimit,origin, platform.Now(), item.ID, item.Version, op.DataEpoch, item.Provider, item.Model)
		if err != nil {
			return 0, 0, gatewayruntime.ExecutionStorageError(err)
		}
		count, err := changed.RowsAffected()
		if err != nil {
			return 0, 0, gatewayruntime.ExecutionStorageError(err)
		}
		if count != 1 {
			return 0, 0, platform.NewError(platform.CodeVersionMismatch, false, "目录模型CAS已变化")
		}
		if err := SaveCatalogPrice(tx, op, item.ID, item.Version+1, remote.Price,origin); err != nil {
			return 0, 0, err
		}
		if _, err := tx.Tx.ExecContext(tx.Context, `INSERT INTO model_catalog_bindings(model_id,data_epoch,provider_id,source_model_id,source_hash) VALUES(?,?,?,?,?) ON CONFLICT(model_id,data_epoch) DO UPDATE SET provider_id=excluded.provider_id,source_model_id=excluded.source_model_id,source_hash=excluded.source_hash`, item.ID, op.DataEpoch, item.Provider, item.Model, sourceHash); err != nil {
			return 0, 0, gatewayruntime.ExecutionStorageError(err)
		}
		payload := struct {
			ModelID string `json:"modelId"`
			Version int    `json:"version"`
			Epoch   string `json:"dataEpoch"`
		}{item.ID, item.Version + 1, op.DataEpoch}
		if err := gatewayruntime.EnqueueExecutionEvent(tx, op, "model.changed", item.ID, item.Version+1, payload); err != nil {
			return 0, 0, err
		}
		updated++
	}
	return updated, preserved, nil
}
func SaveCatalogPrice(tx *gatewayruntime.ExecutionTransaction, op OperationContext, modelID string, version int, price billing.ManualPrice, origin string) error {
	now := time.Now().UTC()
	snapshot, err := billing.MakePriceSnapshot(tx.Context, op, modelID, version, origin, now, price)
	if err != nil {
		return err
	}
	raw, _ := platform.CanonicalJSON(snapshot)
	_, err = tx.Tx.ExecContext(tx.Context, `INSERT INTO price_book_entries(price_snapshot_id,model_id,model_version,data_epoch,source,effective_seconds,effective_nanos,snapshot_json,snapshot_hash) VALUES(?,?,?,?,?,?,?,?,?)`, snapshot.PriceSnapshotId, modelID, version, op.DataEpoch,origin, now.Unix(), now.Nanosecond(), string(raw), platform.HashBytes(raw))
	return gatewayruntime.ExecutionStorageError(err)
}
func NewCatalogRefreshHandlers() map[string]http.Handler {
	return map[string]http.Handler{"refreshModelCatalog": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		var input RefreshModelCatalogInput
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "目录刷新输入不可读"), op.RequestID))
			return
		}
		if key := r.Header.Get("Idempotency-Key"); key != "" && key != input.IdempotencyKey {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "目录刷新幂等头不一致"), op.RequestID))
			return
		}
		result, err := RefreshModelCatalog(r.Context(), op, input)
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})}
}
