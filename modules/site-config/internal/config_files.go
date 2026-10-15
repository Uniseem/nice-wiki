package siteconfig

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

const configSchemaVersion = 1

type configDocument struct {
	SchemaVersion int            `json:"schemaVersion"`
	Config        gen.SiteConfig `json:"config"`
}

type configJournalRecord struct {
	SchemaVersion    int            `json:"schemaVersion"`
	State            string         `json:"state"`
	SiteID           gen.Identifier `json:"siteId"`
	RevisionID       gen.Identifier `json:"revisionId"`
	Object           gen.ObjectRef  `json:"object"`
	ExpectedRevision gen.Revision   `json:"expectedRevision"`
	OperationID      gen.Identifier `json:"operationId"`
	ActorID          gen.Identifier `json:"actorId"`
	RequestHash      gen.Hash       `json:"requestHash"`
	DataEpoch        gen.Identifier `json:"dataEpoch"`
	Initialization   bool           `json:"initialization"`
	ChangedSections  []string       `json:"changedSections"`
}

type WriteConfigRevisionInput struct {
	Config     gen.SiteConfig `json:"config"`
	RevisionId gen.Identifier `json:"revisionId"`
}

type WriteConfigRevisionResult struct {
	Object gen.ObjectRef `json:"object"`
}

type CommitConfigHeadInput struct {
	Object           gen.ObjectRef `json:"object"`
	ExpectedRevision gen.Revision  `json:"expectedRevision"`
	RequestHash      gen.Hash      `json:"requestHash"`
	Initialization   bool          `json:"initialization"`
	ChangedSections  []string      `json:"changedSections"`
}

type CommitConfigHeadResult struct {
	Config gen.SiteConfig `json:"config"`
}

type RecoverConfigJournalInput struct{}
type RecoverConfigJournalResult struct {
	CommittedCount   int `json:"committedCount"`
	QuarantinedCount int `json:"quarantinedCount"`
}

var configSchema struct {
	sync.Once
	schema *openapi3.Schema
	err    error
}

// WriteConfigRevision 持久不可变文件及PREPARED；此时尚不公开配置。
func WriteConfigRevision(ctx context.Context, op platform.OperationContext, input WriteConfigRevisionInput) (WriteConfigRevisionResult, error) {
	store := currentActiveStore()
	if store == nil {
		return WriteConfigRevisionResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	if !configPathID(input.Config.SiteId) || !configPathID(input.RevisionId) {
		return WriteConfigRevisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置文件标识不合法")
	}
	raw, err := canonicalConfigJSON(configDocument{SchemaVersion: configSchemaVersion, Config: input.Config})
	if err != nil {
		return WriteConfigRevisionResult{}, err
	}
	object := gen.ObjectRef{Owner: "site-config", ObjectId: input.RevisionId, Version: input.Config.Revision,
		Sha256: platform.HashBytes(raw), ByteSize: len(raw), MediaType: "application/json"}
	if _, err := decodeConfigDocument(raw, object); err != nil {
		return WriteConfigRevisionResult{}, err
	}
	store.configMu.Lock()
	defer store.configMu.Unlock()
	if err := writeConfigFile(configObjectPath(store.activeRoot, input.Config.SiteId, input.RevisionId), raw, true); err != nil {
		return WriteConfigRevisionResult{}, err
	}
	prepared := configJournalRecord{SchemaVersion: configSchemaVersion, State: "PREPARED", SiteID: input.Config.SiteId,
		RevisionID: input.RevisionId, Object: object}
	if err := writeConfigRecord(store.activeRoot, prepared); err != nil {
		return WriteConfigRevisionResult{}, err
	}
	return WriteConfigRevisionResult{Object: object}, nil
}

// CommitConfigHead 先持久不可反悔文件决定，再更新head和SQL事务。
// SQL失败只可补完；COMMITTED先于SQL不等于已对外发布。
func CommitConfigHead(ctx context.Context, op platform.OperationContext, input CommitConfigHeadInput) (CommitConfigHeadResult, error) {
	store := currentActiveStore()
	if store == nil {
		return CommitConfigHeadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.configMu.Lock()
	defer store.configMu.Unlock()
	prepared, err := readConfigRecord(configRecordPath(store.activeRoot, input.Object.ObjectId, "PREPARED"))
	if err != nil {
		return CommitConfigHeadResult{}, err
	}
	if prepared.State != "PREPARED" || prepared.Object != input.Object {
		return CommitConfigHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置PREPARED与对象不一致")
	}
	config, err := readConfigDocument(store.activeRoot, prepared)
	if err != nil {
		return CommitConfigHeadResult{}, err
	}
	record := prepared
	record.State, record.ExpectedRevision = "COMMITTED", input.ExpectedRevision
	record.OperationID, record.ActorID, record.RequestHash = op.OperationID, op.Actor.SubjectID, input.RequestHash
	record.DataEpoch, record.Initialization, record.ChangedSections = op.DataEpoch, input.Initialization, input.ChangedSections
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return CommitConfigHeadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置提交事务失败")
	}
	defer tx.Rollback()
	// 在不可反悔文件决定前先取得SQLite写锁；跨进程CAS不能只靠Go互斥锁。
	if _, err := tx.ExecContext(ctx, `UPDATE site_config_state SET singleton=singleton WHERE singleton=1`); err != nil {
		return CommitConfigHeadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "取得配置提交写锁失败")
	}
	paths, err := filepath.Glob(filepath.Join(store.activeRoot, "journal", "config", "*.committed.json"))
	if err != nil {
		return CommitConfigHeadResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置决定目录不可读")
	}
	for _, path := range paths {
		decided, err := readConfigRecord(path)
		if err != nil {
			return CommitConfigHeadResult{}, err
		}
		if decided.Object.Version == record.Object.Version && decided.RevisionID != record.RevisionID {
			return CommitConfigHeadResult{}, platform.NewError(platform.CodeConflict, false, "该配置水位已有不可反悔提交决定")
		}
	}
	if err := store.checkConfigCommit(ctx, tx, record, config); err != nil {
		return CommitConfigHeadResult{}, err
	}
	if _, err := CheckWriteEpoch(ctx, op, CheckWriteEpochInput{DataEpoch: op.DataEpoch}); err != nil {
		return CommitConfigHeadResult{}, err
	}
	if err := writeConfigRecord(store.activeRoot, record); err != nil {
		return CommitConfigHeadResult{}, err
	}
	if err := writeConfigHead(store.activeRoot, record); err != nil {
		return CommitConfigHeadResult{}, err
	}
	if err := store.applyConfigCommit(ctx, tx, record, config); err != nil {
		return CommitConfigHeadResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CommitConfigHeadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "提交配置水位失败，须按原操作恢复")
	}
	return CommitConfigHeadResult{Config: config}, nil
}

// RecoverConfigJournal 在业务入口开放前只回放明确COMMITTED决定。
// PREPARED仅有初始化commit决定时留待Resume，其余写隔离标记。
func RecoverConfigJournal(ctx context.Context, op platform.OperationContext, input RecoverConfigJournalInput) (RecoverConfigJournalResult, error) {
	if op.CallerService != "site-config" || op.Actor.Kind != "service" {
		return RecoverConfigJournalResult{}, platform.NewError(platform.CodeForbidden, false, "配置恢复仅允许本域受控启动")
	}
	store := currentActiveStore()
	if store == nil {
		return RecoverConfigJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	store.configMu.Lock()
	defer store.configMu.Unlock()
	paths, err := filepath.Glob(filepath.Join(store.activeRoot, "journal", "config", "*.committed.json"))
	if err != nil {
		return RecoverConfigJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置journal路径不合法")
	}
	records := make([]configJournalRecord, 0, len(paths))
	for _, path := range paths {
		record, err := readConfigRecord(path)
		if err != nil {
			return RecoverConfigJournalResult{}, err
		}
		if record.State != "COMMITTED" || record.DataEpoch != op.DataEpoch {
			return RecoverConfigJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置COMMITTED决定代次不一致")
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Object.Version < records[j].Object.Version })
	result := RecoverConfigJournalResult{}
	committed := map[string]bool{}
	for i, record := range records {
		if record.ExpectedRevision != gen.Revision(i) || record.Object.Version != gen.Revision(i+1) {
			return result, platform.NewError(platform.CodeIntegrityError, false, "配置COMMITTED决定水位不连续")
		}
		config, err := readConfigDocument(store.activeRoot, record)
		if err != nil {
			return result, err
		}
		prepared, err := readConfigRecord(configRecordPath(store.activeRoot, record.RevisionID, "PREPARED"))
		if err != nil || prepared.State != "PREPARED" || prepared.SiteID != record.SiteID || prepared.Object != record.Object {
			return result, platform.NewError(platform.CodeIntegrityError, false, "配置COMMITTED缺少一致的PREPARED")
		}
		tx, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			return result, platform.NewError(platform.CodeDependencyUnavailable, true, "打开配置恢复事务失败")
		}
		if err := store.applyConfigCommit(ctx, tx, record, config); err != nil {
			tx.Rollback()
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, platform.NewError(platform.CodeDependencyUnavailable, true, "提交配置恢复水位失败")
		}
		committed[record.RevisionID] = true
		result.CommittedCount++
	}
	if len(records) > 0 {
		if err := writeConfigHead(store.activeRoot, records[len(records)-1]); err != nil {
			return result, err
		}
	}
	paths, err = filepath.Glob(filepath.Join(store.activeRoot, "journal", "config", "*.prepared.json"))
	if err != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "配置PREPARED路径不合法")
	}
	for _, path := range paths {
		record, err := readConfigRecord(path)
		if err != nil {
			return result, err
		}
		if record.State != "PREPARED" {
			return result, platform.NewError(platform.CodeIntegrityError, false, "配置PREPARED阶段不合法")
		}
		if committed[record.RevisionID] {
			continue
		}
		var pending int
		err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_operations o JOIN initialization_decisions d ON d.operation_id=o.operation_id WHERE o.revision_id=? AND d.decision='commit' AND o.site_id=? AND o.data_epoch=?`, record.RevisionID, record.SiteID, op.DataEpoch).Scan(&pending)
		if err != nil {
			return result, platform.NewError(platform.CodeDependencyUnavailable, true, "读取配置准备决定失败")
		}
		if pending == 0 {
			raw, err := canonicalConfigJSON(record)
			if err != nil {
				return result, err
			}
			if err := writeConfigFile(filepath.Join(store.activeRoot, "quarantine", "config", record.RevisionID+".json"), raw, true); err != nil {
				return result, err
			}
			result.QuarantinedCount++
		}
	}
	if err := store.verifyConfigHead(ctx); err != nil {
		return result, err
	}
	store.stateMu.Lock()
	store.configRecovered = true
	store.stateMu.Unlock()
	return result, nil
}

func (s *ActiveStore) checkConfigCommit(ctx context.Context, tx *sql.Tx, record configJournalRecord, config gen.SiteConfig) error {
	var epoch string
	var site sql.NullString
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,site_id,config_revision FROM site_config_state WHERE singleton=1`).Scan(&epoch, &site, &revision); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "配置当前水位不可读")
	}
	if epoch != record.DataEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "配置dataEpoch已变化")
	}
	if record.SiteID != config.SiteId || record.Object.Version != record.ExpectedRevision+1 || (site.Valid && site.String != record.SiteID) {
		return platform.NewError(platform.CodeIntegrityError, false, "配置提交站点或修订不一致")
	}
	if revision != int64(record.ExpectedRevision) && revision != int64(record.Object.Version) {
		return platform.NewError(platform.CodeVersionMismatch, false, "配置expectedRevision已变化")
	}
	var receiptActor, receiptSite, receiptEpoch, receiptHash string
	if err := tx.QueryRowContext(ctx, `SELECT actor_id,site_id,data_epoch,request_hash FROM config_operations WHERE operation_id=?`, record.OperationID).Scan(&receiptActor, &receiptSite, &receiptEpoch, &receiptHash); err != nil || receiptActor != record.ActorID || receiptSite != record.SiteID || receiptEpoch != record.DataEpoch || receiptHash != record.RequestHash {
		return platform.NewError(platform.CodeIntegrityError, false, "配置决定缺少一致的原始操作回执")
	}
	if revision == int64(record.Object.Version) {
		var revisionID, hash string
		err := tx.QueryRowContext(ctx, `SELECT revision_id,sha256 FROM config_revision_index WHERE site_id=? AND revision=?`, record.SiteID, revision).Scan(&revisionID, &hash)
		if err != nil || revisionID != record.RevisionID || hash != record.Object.Sha256 {
			return platform.NewError(platform.CodeConflict, false, "配置修订已由另一操作提交")
		}
	}
	return nil
}

func (s *ActiveStore) applyConfigCommit(ctx context.Context, tx *sql.Tx, record configJournalRecord, config gen.SiteConfig) error {
	var epoch string
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,config_revision FROM site_config_state WHERE singleton=1`).Scan(&epoch, &revision); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "配置恢复水位不可读")
	}
	if epoch != record.DataEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "配置决定不属于当前dataEpoch")
	}
	if revision >= int64(record.Object.Version) {
		var hash, revisionID string
		if err := tx.QueryRowContext(ctx, `SELECT sha256,revision_id FROM config_revision_index WHERE site_id=? AND revision=?`, record.SiteID, record.Object.Version).Scan(&hash, &revisionID); err != nil || hash != record.Object.Sha256 || revisionID != record.RevisionID {
			return platform.NewError(platform.CodeIntegrityError, false, "配置历史索引与COMMITTED决定不一致")
		}
		return verifyConfigReceipt(ctx, tx, record)
	}
	if err := s.checkConfigCommit(ctx, tx, record, config); err != nil {
		return err
	}
	var receiptHash, actorID, siteID string
	err := tx.QueryRowContext(ctx, `SELECT request_hash,actor_id,site_id FROM config_operations WHERE operation_id=?`, record.OperationID).Scan(&receiptHash, &actorID, &siteID)
	if err != nil || receiptHash != record.RequestHash || actorID != record.ActorID || siteID != record.SiteID {
		return platform.NewError(platform.CodeIntegrityError, false, "配置决定缺少一致的原始操作回执")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO config_revision_index(site_id,revision_id,object_id,sha256,byte_size,revision,committed_at) VALUES(?,?,?,?,?,?,?)`,
		record.SiteID, record.RevisionID, record.Object.ObjectId, record.Object.Sha256, record.Object.ByteSize, config.Revision, config.UpdatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "写入配置修订索引失败")
	}
	result, err := tx.ExecContext(ctx, `UPDATE site_config_state SET site_id=?,config_revision=?,config_revision_id=?,updated_at=? WHERE singleton=1 AND data_epoch=? AND config_revision=?`,
		record.SiteID, config.Revision, record.RevisionID, config.UpdatedAt.UTC().Format(time.RFC3339Nano), record.DataEpoch, record.ExpectedRevision)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "保存配置水位失败")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeVersionMismatch, false, "配置CAS失败")
	}
	operation := platform.OperationContext{OperationID: record.OperationID, DataEpoch: record.DataEpoch}
	if _, err := PublishConfigEvent(ctx, operation, tx, PublishConfigEventInput{EventId: record.RevisionID, Event: ConfigChanged{
		SiteId: config.SiteId, ConfigRevision: config.Revision, BaseLanguage: config.BaseLanguage, HomeRevision: config.Home.Revision,
		ChangedSections: record.ChangedSections, OccurredAt: config.UpdatedAt,
	}}); err != nil {
		return err
	}
	if record.Initialization {
		state, _, err := s.readInitializationRecordBeforeConfig(ctx, tx, record)
		if err != nil {
			return err
		}
		state.Phase, state.ConfigRevision, state.Revision = gen.InitializationStatePhaseConfigCommitted, config.Revision, state.Revision+1
		state.SafeError, state.UpdatedAt = nil, time.Now().UTC()
		raw, err := json.Marshal(state)
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "编码配置初始化状态失败")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE initialization_state SET revision=?,state_json=? WHERE singleton=1 AND operation_id=?`, state.Revision, string(raw), record.OperationID); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "保存配置初始化阶段失败")
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE config_operations SET state='committed',result_revision=? WHERE operation_id=? AND request_hash=?`, config.Revision, record.OperationID, record.RequestHash); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "保存配置回执失败")
	}
	return nil
}

// 此时SQL配置水位刚推进，协调JSON仍是preparing，故不走跨表水位校验。
func (s *ActiveStore) readInitializationRecordBeforeConfig(ctx context.Context, tx *sql.Tx, record configJournalRecord) (gen.InitializationState, bool, error) {
	var raw, decision, siteID string
	var configJSON, configHash sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT s.state_json,d.decision,d.site_id,d.prepared_config_json,d.prepared_config_hash FROM initialization_state s JOIN initialization_decisions d ON d.operation_id=s.operation_id WHERE s.operation_id=?`, record.OperationID).Scan(&raw, &decision, &siteID, &configJSON, &configHash); err != nil {
		return gen.InitializationState{}, false, platform.NewError(platform.CodeIntegrityError, false, "初始化commit决定缺失")
	}
	state, err := decodeInitializationState(raw)
	if err != nil {
		return state, false, err
	}
	if err := validateInitializationMaterial(state, record.OperationID, siteID, decision, configJSON, configHash); err != nil {
		return state, false, err
	}
	if state.Decision != gen.Commit || state.ConfigRevision != record.ExpectedRevision || *state.SiteId != record.SiteID {
		return state, false, platform.NewError(platform.CodeIntegrityError, false, "初始化决定与配置提交不一致")
	}
	return state, false, nil
}

func verifyConfigReceipt(ctx context.Context, tx *sql.Tx, record configJournalRecord) error {
	var actor, site, hash, state string
	var revision sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT actor_id,site_id,request_hash,state,result_revision FROM config_operations WHERE operation_id=?`, record.OperationID).Scan(&actor, &site, &hash, &state, &revision)
	if err != nil || actor != record.ActorID || site != record.SiteID || hash != record.RequestHash || state != "committed" || !revision.Valid || revision.Int64 != int64(record.Object.Version) {
		return platform.NewError(platform.CodeIntegrityError, false, "配置提交回执与文件决定不一致")
	}
	return nil
}

func (s *ActiveStore) verifyConfigHead(ctx context.Context) error {
	var siteID, revisionID sql.NullString
	var revision int64
	if err := s.db.QueryRowContext(ctx, `SELECT site_id,config_revision_id,config_revision FROM site_config_state WHERE singleton=1`).Scan(&siteID, &revisionID, &revision); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "配置head水位不可读")
	}
	if revision == 0 {
		return nil
	}
	if !siteID.Valid || !configPathID(siteID.String) || !revisionID.Valid {
		return platform.NewError(platform.CodeIntegrityError, false, "配置head标识缺失")
	}
	head, err := readConfigRecord(filepath.Join(s.activeRoot, "settings", siteID.String, "head.json"))
	if err != nil {
		return err
	}
	committed, err := readConfigRecord(configRecordPath(s.activeRoot, revisionID.String, "COMMITTED"))
	if err != nil {
		return err
	}
	left, _ := json.Marshal(head)
	right, _ := json.Marshal(committed)
	if !bytes.Equal(left, right) || head.State != "COMMITTED" || head.Object.Version != gen.Revision(revision) {
		return platform.NewError(platform.CodeIntegrityError, false, "配置head与COMMITTED水位不一致")
	}
	_, err = readConfigDocument(s.activeRoot, head)
	return err
}

func readConfigDocument(root string, record configJournalRecord) (gen.SiteConfig, error) {
	if !configPathID(record.SiteID) || !configPathID(record.RevisionID) {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置对象路径标识不合法")
	}
	raw, err := os.ReadFile(configObjectPath(root, record.SiteID, record.RevisionID))
	if err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置权威对象缺失")
	}
	config, err := decodeConfigDocument(raw, record.Object)
	if err != nil {
		return gen.SiteConfig{}, err
	}
	if config.SiteId != record.SiteID {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置对象站点不一致")
	}
	return config, nil
}

func decodeConfigDocument(raw []byte, object gen.ObjectRef) (gen.SiteConfig, error) {
	if object.Owner != "site-config" || object.MediaType != "application/json" || len(raw) != object.ByteSize || platform.HashBytes(raw) != object.Sha256 {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置对象hash或大小不一致")
	}
	var document configDocument
	if err := decodeConfigJSON(raw, &document); err != nil {
		return gen.SiteConfig{}, err
	}
	if document.SchemaVersion != configSchemaVersion {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置文件schema版本不支持")
	}
	configSchema.Do(func() {
		spec, err := gen.GetSwagger()
		if err != nil {
			configSchema.err = err
			return
		}
		configSchema.schema = spec.Components.Schemas["SiteConfig"].Value
	})
	if configSchema.err != nil || configSchema.schema == nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置schema不可用")
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置文件JSON不合法")
	}
	if err := configSchema.schema.VisitJSON(value["config"]); err != nil {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置文件不符合权威schema")
	}
	if document.Config.Revision != object.Version || object.Version < 1 || !document.Config.Initialized {
		return gen.SiteConfig{}, platform.NewError(platform.CodeIntegrityError, false, "配置文件修订或初始化标记不合法")
	}
	return document.Config, nil
}

func readConfigRecord(path string) (configJournalRecord, error) {
	var record configJournalRecord
	raw, err := os.ReadFile(path)
	if err != nil {
		return record, platform.NewError(platform.CodeIntegrityError, false, "配置journal记录缺失")
	}
	if err := decodeConfigJSON(raw, &record); err != nil {
		return record, err
	}
	canonical, err := canonicalConfigJSON(record)
	if err != nil {
		return record, err
	}
	if !bytes.Equal(raw, canonical) {
		return record, platform.NewError(platform.CodeIntegrityError, false, "配置journal不是完整规范记录")
	}
	if record.SchemaVersion != configSchemaVersion || (record.State != "PREPARED" && record.State != "COMMITTED") || !configPathID(record.SiteID) || !configPathID(record.RevisionID) || record.Object.ObjectId != record.RevisionID || record.Object.Owner != "site-config" || !platform.ValidHash(record.Object.Sha256) || record.Object.ByteSize < 1 || record.Object.Version < 1 {
		return record, platform.NewError(platform.CodeIntegrityError, false, "配置journal字段不合法")
	}
	if record.State == "COMMITTED" && (!platform.ValidIdentifier(record.OperationID) || !platform.ValidIdentifier(record.ActorID) || !platform.ValidHash(record.RequestHash) || !platform.ValidIdentifier(record.DataEpoch) || record.ExpectedRevision+1 != record.Object.Version || len(record.ChangedSections) == 0 || len(record.ChangedSections) > 6) {
		return record, platform.NewError(platform.CodeIntegrityError, false, "配置COMMITTED决定不完整")
	}
	if record.State == "COMMITTED" {
		sections := map[string]bool{}
		for _, section := range record.ChangedSections {
			switch section {
			case "identity", "home", "seo", "security", "limits", "language":
			default:
				return record, platform.NewError(platform.CodeIntegrityError, false, "配置变更组不合法")
			}
			if sections[section] {
				return record, platform.NewError(platform.CodeIntegrityError, false, "配置变更组重复")
			}
			sections[section] = true
		}
	}
	return record, nil
}

func decodeConfigJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "配置持久JSON格式不合法")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return platform.NewError(platform.CodeIntegrityError, false, "配置持久JSON包含多余内容")
	}
	return nil
}

func canonicalConfigJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "配置无法序列化")
	}
	var parsed any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "配置无法规范序列化")
	}
	raw, err = json.Marshal(parsed)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "配置无法规范序列化")
	}
	return raw, nil
}

func configPathID(id string) bool { return platform.ValidIdentifier(id) && id != "." && id != ".." }
func configObjectPath(root, siteID, revisionID string) string {
	return filepath.Join(root, "settings", siteID, revisionID+".json")
}
func configRecordPath(root, revisionID, state string) string {
	return filepath.Join(root, "journal", "config", revisionID+"."+strings.ToLower(state)+".json")
}

func writeConfigRecord(root string, record configJournalRecord) error {
	raw, err := canonicalConfigJSON(record)
	if err != nil {
		return err
	}
	return writeConfigFile(configRecordPath(root, record.RevisionID, record.State), raw, true)
}
func writeConfigHead(root string, record configJournalRecord) error {
	raw, err := canonicalConfigJSON(record)
	if err != nil {
		return err
	}
	return writeConfigFile(filepath.Join(root, "settings", record.SiteID, "head.json"), raw, false)
}

// writeConfigFile 同FS临时写、fsync后发布，再fsync父目录。
// 不可变对象以Link排他发布；head以Rename原子替换。
func writeConfigFile(path string, raw []byte, immutable bool) error {
	dir := filepath.Dir(path)
	if err := ensureConfigDirectory(dir); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置临时文件不可写")
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "写入配置文件失败")
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置文件fsync失败")
	}
	if err := file.Close(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "关闭配置文件失败")
	}
	if immutable {
		err = os.Link(name, path)
		if errors.Is(err, os.ErrExist) {
			existing, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(existing, raw) {
				return platform.NewError(platform.CodeIntegrityError, false, "不可变配置文件被异内容复用")
			}
		} else if err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "发布不可变配置文件失败")
		}
	} else if err := os.Rename(name, path); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "原子发布配置head失败")
	}
	return syncConfigDirectory(dir)
}

func syncConfigDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置目录不可读")
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置目录fsync失败")
	}
	return nil
}

func ensureConfigDirectory(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置目录不可读")
	}
	if err := ensureConfigDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "配置目录不可写")
	}
	return syncConfigDirectory(filepath.Dir(path))
}
