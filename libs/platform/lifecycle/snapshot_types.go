// 公共快照机制不包含业务表规则，安全截点与迁移由真实域回调证明。
package lifecycle

import (
 "context"
 "io"
 "encoding/json"
 "time"
 "nice-wiki/libs/platform"
)

const MaxArchiveBytes int64 = 20 << 30
const MaxManifestBytes int64 = 16 << 20
const MaxArchiveFiles = 100000

// DomainSnapshotManifest 对应common DomainSnapshotManifest V1；最终类型由ROOT统一放平台，不让域复制。
type DomainSnapshotManifest struct {
 Format string `json:"format"`
 SchemaVersion int `json:"schemaVersion"`
 Owner string `json:"owner"`
 BackupID string `json:"backupId"`
 BarrierID string `json:"barrierId"`
 SnapshotID string `json:"snapshotId"`
 SourceEpoch string `json:"sourceEpoch"`
 DataSchemaVersion int `json:"dataSchemaVersion"`
 RevisionWatermark int64 `json:"revisionWatermark"`
 ContainsSecrets bool `json:"containsSecrets"`
 Files []DomainSnapshotFile `json:"files"`
 Objects []DomainSnapshotObject `json:"objects"`
 Publications []SnapshotPublishedRevisionRef `json:"publications"`
 References []DomainSnapshotReference `json:"references"`
 Assets []platform.AssetVersionRef `json:"assets"`
 Invocations []SnapshotInvocationRef `json:"invocations"`
}
type DomainSnapshotFile struct{Path string `json:"path"`;SHA256 string `json:"sha256"`;ByteSize int64 `json:"byteSize"`}
type DomainSnapshotObject struct{Reference platform.ObjectRef `json:"reference"`;Path string `json:"path"`}
type SnapshotPublishedRevisionRef struct{DocumentID string `json:"documentId"`;PublishedRevisionID string `json:"publishedRevisionId"`;DataEpoch string `json:"dataEpoch"`;Revision int64 `json:"revision"`;ContentHash string `json:"contentHash"`;SchemaVersion int `json:"schemaVersion"`}
type SnapshotInvocationRef struct{InvocationID string `json:"invocationId"`;RootOperationID string `json:"rootOperationId"`;DataEpoch string `json:"dataEpoch"`}
type DomainSnapshotReference struct{Kind string `json:"kind"`;Object *platform.ObjectRef `json:"object,omitempty"`;Publication *SnapshotPublishedRevisionRef `json:"publication,omitempty"`;Asset *platform.AssetVersionRef `json:"asset,omitempty"`;Invocation *SnapshotInvocationRef `json:"invocation,omitempty"`}
// V1的canonical字段集合保持原样；V2必须显式有完整真实库存，空数组也是事实。
func(m DomainSnapshotManifest)MarshalJSON()([]byte,error){type wire DomainSnapshotManifest;if m.SchemaVersion==2 {if m.Assets==nil||m.Invocations==nil{return nil,integrity("V2真实资源与调用库存缺失")};return json.Marshal(wire(m))};if len(m.Assets)!=0||len(m.Invocations)!=0{return nil,integrity("V1不能声明V2库存")};for _,ref:=range m.References{if ref.Asset!=nil||ref.Invocation!=nil{return nil,integrity("V1不能声明V2类型引用")}};raw,err:=json.Marshal(wire(m));if err!=nil{return nil,err};var fields map[string]json.RawMessage;if err=json.Unmarshal(raw,&fields);err!=nil{return nil,err};delete(fields,"assets");delete(fields,"invocations");return json.Marshal(fields)}
type SnapshotRef struct{Owner string `json:"owner"`;SnapshotID string `json:"snapshotId"`;DataEpoch string `json:"dataEpoch"`;DataSchemaVersion int `json:"dataSchemaVersion"`;Object platform.ObjectRef `json:"object"`;RevisionWatermark int64 `json:"revisionWatermark"`;ContainsSecrets bool `json:"containsSecrets"`}

// BarrierInput来自已鉴权的data-protection服务调用，不从客户端自报actor猜授权。
type SnapshotBarrierInput struct{BarrierID string `json:"barrierId"`;OperationID string `json:"operationId"`;Epoch string `json:"-"`;Mode string `json:"mode"`;Deadline time.Time `json:"deadline"`}
type SnapshotBarrierStatus struct{BarrierID string `json:"barrierId"`;Owner string `json:"owner"`;State string `json:"state"`;DataEpoch string `json:"dataEpoch"`;RevisionWatermark int64 `json:"revisionWatermark"`;SealProofHash string `json:"sealProofHash,omitempty"`}
type SnapshotSafeCut struct{BarrierID string;OperationID string;Epoch string;Watermark int64}
type SnapshotExportInput struct{BackupID string `json:"backupId"`;BarrierID string `json:"barrierId"`;DataEpoch string `json:"dataEpoch"`;IncludeSecrets bool `json:"includeSecrets"`;SnapshotID string `json:"-"`}
type SnapshotExportResult struct{Snapshot SnapshotRef `json:"snapshot"`;Checksum string `json:"checksum"`}
// StageInput不含任意URL/系统路径，归档通过ROOT版本化受控流端口取得。
type SnapshotStageInput struct{RestoreID string `json:"restoreId"`;Snapshot SnapshotRef `json:"snapshot"`;BarrierID string `json:"barrierId"`;TargetDataEpoch string `json:"targetDataEpoch"`;Checksum string `json:"checksum"`}
type SnapshotStageResult struct{RestoreID string `json:"restoreId"`;Owner string `json:"owner"`;SnapshotID string `json:"snapshotId"`;TargetDataEpoch string `json:"targetDataEpoch"`;Checksum string `json:"checksum"`;Staged bool `json:"staged"`;StageID string `json:"stageId"`;ManifestHash string `json:"manifestHash"`}
type SnapshotStageReceipt struct{RestoreID string `json:"restoreId"`;StageID string `json:"stageId"`;SnapshotID string `json:"snapshotId"`;ManifestHash string `json:"manifestHash,omitempty"`;SourceEpoch string `json:"sourceEpoch"`;TargetEpoch string `json:"targetEpoch"`;InputHash string `json:"inputHash"`;State string `json:"state"`}
type SnapshotDiscardInput struct{RestoreID string `json:"restoreId"`;Scope string `json:"scope"`}
type SnapshotDiscardResult struct{RestoreID string `json:"restoreId"`;Scope string `json:"scope"`;Removed bool `json:"removed"`}

// Config的回调仅由各域启动装配；缺回调失败，不把未知inventory/费用事实冒充空数组或安全。
type SnapshotSwapEvidence struct{Known bool;Switched bool;RolledBack bool;Aborted bool;StageID string;ManifestHash string;HostManifestHash string;HostEpoch string;TargetEpoch string}

// SnapshotTree穷尽完整目标文件和目录，包括空目录。排序后canonical hash为激活manifestHash。
type SnapshotTree struct{Files []DomainSnapshotFile `json:"files"`;Directories []string `json:"directories"`}

type SnapshotConfig struct {
 Owner string
 DataRoot string
 OpenedDataEpoch string
 Runtime func(context.Context)(platform.RuntimeView,error)
 ValidateManifest func([]byte)error
 // Drain只收尾既有执行：原费用结算/取消/outbox排空。Cleanup只在回调有效期发专用既有执行窗口，不允许新产品操作。
 BeginDrain func(context.Context,*SnapshotCleanup)error
 Drain func(context.Context,*SnapshotCleanup)error
 // Watermark在draining、旧business及cleanup都为0后读取真实业务修订。
 Watermark func(context.Context)(int64,error)
 // ObserveWork必须读本域真实全部未发布outbox与已批准旧执行；包含未来重试/leased/quarantine，未知返回错误。
 ObserveWork func(context.Context)(SnapshotWorkState,error)
 // SealWork在本域持久事务内CAS outboxRevision+workRevision，关闭其它持久写入口；零值不是证明。
 SealWork func(context.Context,SnapshotWorkState,string)error
 // ResumeWork只重开旧执行清理，不开启新业务/付费。
 ResumeWork func(context.Context,string)error
 // VerifyGlobalSeal校验真实协调器阶段/13域封口回执/jobs最终水位，不能仅信请求自报。
 VerifyGlobalSeal func(context.Context,SnapshotSealProof,string)error
 CleanupRoutes []platform.CleanupRoute
 // 验证drain回调提供的原对象确为已批准旧执行，禁止将协调器随机root当原付费root。
 AuthorizeCleanupOrigin func(context.Context,platform.OperationContext)error

 // Capture在safe固定水位后，把真实完整inventory的普通文件写到workspace/files；SQL一致备份、敏感数据选择由owner明确完成。
 // 返回完整真实objects/publications/references，不允许用[]代替未知；不读其他域私有目录、不把control/vault递归复制进去。
 Capture func(context.Context,SnapshotSafeCut,string,SnapshotExportInput)(DomainSnapshotManifest,error)
 // MigrateStage只在完整认证归档解包后建立目标epoch索引。不能通用替换正文块ID/付费对象/引用，也不激活。
 MigrateStage func(context.Context,DomainSnapshotManifest,string,string)error
 // ValidateStaged在迁移后及重启续接时证明本域目标schema/epoch正确；普通旧文件或对象不能伪装业务回执。
 ValidateStaged func(context.Context,DomainSnapshotManifest,string,string)error
 // StageBudget由owner按真实源清单给迁移的完整目标/临时文件峰值上界，未知必须返回错误。
 StageBudget func(context.Context,DomainSnapshotManifest)(int64,error)
}

// API的具体状态/IO实现分barrier.go/archive.go/manager.go；草稿外不会自行注册HTTP或伪造跨域回执。
type SnapshotAPI interface {
 AdmitBusiness(context.Context)(context.Context,func(),error)
 WriteWindow(context.Context)(time.Time,error)
 PrepareBarrier(context.Context,SnapshotBarrierInput)(SnapshotBarrierStatus,error)
 DrainBarrier(context.Context,string)(SnapshotBarrierStatus,error)
 ObserveBarrier(context.Context,string)(SnapshotBarrierObservation,error)
 SealBarrier(context.Context,SnapshotSealProof)(SnapshotBarrierStatus,error)
 ResumeDrain(context.Context,string,string)(SnapshotBarrierStatus,error)
 EnterBarrier(context.Context,SnapshotBarrierInput)(SnapshotBarrierStatus,error)
 ReleaseBarrier(context.Context,string)(SnapshotBarrierStatus,error)
 Export(context.Context,SnapshotExportInput)(SnapshotExportResult,error)
 OpenArchive(context.Context,string,platform.ObjectRef)(io.ReadCloser,error)
 Stage(context.Context,SnapshotStageInput,io.Reader)(SnapshotStageResult,error)
 Discard(context.Context,SnapshotDiscardInput)(SnapshotDiscardResult,error)
}

// Revision是实际SQL变更水位，PendingWork包含旧执行待决事实，冻结queued job不计事件delivery。
type SnapshotWorkState struct{OutboxRevision int64 `json:"outboxRevision"`;WorkRevision int64 `json:"workRevision"`;PendingOutbox int64 `json:"pendingOutbox"`;PendingWork int64 `json:"pendingWork"`}
type SnapshotBarrierObservation struct{Owner string `json:"owner"`;ActivityRevision int64 `json:"activityRevision"`;Work SnapshotWorkState `json:"work"`;RevisionWatermark int64 `json:"revisionWatermark"`;Business int64 `json:"business"`;Cleanup int64 `json:"cleanup"`;FileOps int64 `json:"fileOps"`;DrainCallbacks int64 `json:"drainCallbacks"`}
type SnapshotBrokerObservation struct{Revision int64 `json:"revision"`;PendingDeliveries int64 `json:"pendingDeliveries"`;ActiveLeases int64 `json:"activeLeases"`}
type SnapshotSealReceipt struct{Owner string `json:"owner"`;SealProofHash string `json:"sealProofHash"`}
type SnapshotSealProof struct{BarrierID string `json:"barrierId"`;DataEpoch string `json:"dataEpoch"`;Observations []SnapshotBarrierObservation `json:"observations"`;BrokerBefore SnapshotBrokerObservation `json:"brokerBefore"`;BrokerAfter SnapshotBrokerObservation `json:"brokerAfter"`;NonJobsSeals []SnapshotSealReceipt `json:"nonJobsSeals,omitempty"`;NonJobsProofHash string `json:"nonJobsProofHash,omitempty"`}

// validateSnapshotManifestVersion只核持久清单版本与库存存在性，不重基原V1/V2摘要。
func validateSnapshotManifestVersion(manifest DomainSnapshotManifest)error{
 if manifest.Format!="nice-wiki.domain-snapshot"||(manifest.SchemaVersion!=1&&manifest.SchemaVersion!=2){return snapshotUnsupported("域清单版本不支持")};if manifest.Files==nil||manifest.Objects==nil||manifest.Publications==nil||manifest.References==nil{return integrity("清单真实库存缺失")};if manifest.SchemaVersion==2{if manifest.Assets==nil||manifest.Invocations==nil{return integrity("V2 typed真实库存缺失")}}else{if len(manifest.Assets)!=0||len(manifest.Invocations)!=0{return integrity("V1不能声明V2库存")};for _,reference:=range manifest.References{if reference.Asset!=nil||reference.Invocation!=nil{return integrity("V1不能声明V2类型引用")}}};return nil
}
