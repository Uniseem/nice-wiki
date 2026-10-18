package protection

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/data-protection/internal/gen"
	"sort"
)

const maxBackupManifestBytes = 2 << 20

type SnapshotRef struct {
	SnapshotID        string             `json:"snapshotId"`
	Owner             string             `json:"owner"`
	DataEpoch         string             `json:"dataEpoch"`
	DataSchemaVersion int                `json:"dataSchemaVersion"`
	RevisionWatermark int64              `json:"revisionWatermark"`
	Object            platform.ObjectRef `json:"object"`
	ContainsSecrets   bool               `json:"containsSecrets"`
}
type PublishedRevisionRef struct {
	DocumentID          string `json:"documentId"`
	PublishedRevisionID string `json:"publishedRevisionId"`
	Revision            int64  `json:"revision"`
	ContentHash         string `json:"contentHash"`
	SchemaVersion       int    `json:"schemaVersion"`
	DataEpoch           string `json:"dataEpoch"`
}
type DomainSnapshotFile struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	ByteSize int64  `json:"byteSize"`
}
type DomainSnapshotObject struct {
	Reference platform.ObjectRef `json:"reference"`
	Path      string             `json:"path"`
}
type SnapshotInvocationRef struct {
 InvocationID string `json:"invocationId"`
 RootOperationID string `json:"rootOperationId"`
 DataEpoch string `json:"dataEpoch"`
}
type DomainSnapshotReference struct {
 Asset *platform.AssetVersionRef `json:"asset,omitempty"`
 Invocation *SnapshotInvocationRef `json:"invocation,omitempty"`
	Kind        string                `json:"kind"`
	Object      *platform.ObjectRef   `json:"object,omitempty"`
	Publication *PublishedRevisionRef `json:"publication,omitempty"`
}
type DomainSnapshotManifest struct {
 Assets []platform.AssetVersionRef `json:"assets"`
 Invocations []SnapshotInvocationRef `json:"invocations"`
	Format            string                    `json:"format"`
	SchemaVersion     int                       `json:"schemaVersion"`
	Owner             string                    `json:"owner"`
	BackupID          string                    `json:"backupId"`
	BarrierID         string                    `json:"barrierId"`
	SnapshotID        string                    `json:"snapshotId"`
	SourceEpoch       string                    `json:"sourceEpoch"`
	DataSchemaVersion int                       `json:"dataSchemaVersion"`
	RevisionWatermark int64                     `json:"revisionWatermark"`
	ContainsSecrets   bool                      `json:"containsSecrets"`
	Files             []DomainSnapshotFile      `json:"files"`
	Objects           []DomainSnapshotObject    `json:"objects"`
	Publications      []PublishedRevisionRef    `json:"publications"`
	References        []DomainSnapshotReference `json:"references"`
}
type BackupDomainV1 struct {
	Snapshot     SnapshotRef `json:"snapshot"`
	ManifestHash string      `json:"manifestHash"`
}
type BackupReferenceV1 struct {
	SourceOwner        string                  `json:"sourceOwner"`
	SourceManifestHash string                  `json:"sourceManifestHash"`
	Reference          DomainSnapshotReference `json:"reference"`
}
type BackupManifestV1 struct {
	Format        string              `json:"format"`
	SchemaVersion int                 `json:"schemaVersion"`
	BackupID      string              `json:"backupId"`
	SourceEpoch   string              `json:"sourceEpoch"`
	BarrierID     string              `json:"barrierId"`
	CreatedAt     string              `json:"createdAt"`
	KeyID         string              `json:"keyId"`
	Domains       []BackupDomainV1    `json:"domains"`
	References    []BackupReferenceV1 `json:"references"`
}
type BuildManifestInput struct {
	BackupId        string                   `json:"backupId"`
	SourceEpoch     string                   `json:"sourceEpoch"`
	BarrierId       string                   `json:"barrierId"`
	Snapshots       []SnapshotRef            `json:"snapshots"`
	CreatedAt       string                   `json:"createdAt"`
	KeyId           string                   `json:"keyId"`
	DomainManifests []DomainSnapshotManifest `json:"domainManifests"`
}
type BuildManifestResult struct {
	ManifestJSON string `json:"manifestJSON"`
	ManifestHash string `json:"manifestHash"`
}
type ValidateCrossDomainRefsInput struct {
	ManifestJSON    string                   `json:"manifestJSON"`
	DomainManifests []DomainSnapshotManifest `json:"domainManifests"`
}
type ValidateCrossDomainRefsResult struct {
	Valid  bool            `json:"valid"`
	Issues []gen.SafeIssue `json:"issues"`
}

// bindDomainEvidence 只接已认证完整tar的真实清单；hash绑定原数组，不裁剪域证据。
func bindDomainEvidence(backupID, epoch, barrierID string, snapshots []SnapshotRef, domains []DomainSnapshotManifest) ([]BackupDomainV1, []BackupReferenceV1, error) {
	if len(snapshots) != len(requiredSnapshotOwners) || len(domains) != len(requiredSnapshotOwners) {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "备份缺少完整14域清单")
	}
	byOwner := map[string]DomainSnapshotManifest{}
	for _, domain := range domains {
		if !requiredSnapshotOwners[domain.Owner] {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "备份清单含未知拥有者")
		}
		if _, duplicate := byOwner[domain.Owner]; duplicate {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "备份域清单重复")
		}
		byOwner[domain.Owner] = domain
	}
	ordered := append([]SnapshotRef{}, snapshots...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Owner < ordered[j].Owner })
	bindings := []BackupDomainV1{}
	references := []BackupReferenceV1{}
	seen := map[string]bool{}
	seenRefs := map[string]bool{}
	referenceBytes := 0
	for _, snapshot := range ordered {
		domain, found := byOwner[snapshot.Owner]
		if !found || seen[snapshot.Owner] || snapshot.Object.Owner != snapshot.Owner {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "快照拥有者缺失或重复")
		}
		seen[snapshot.Owner] = true
		if snapshot.DataEpoch != epoch || domain.SourceEpoch != epoch {
			return nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "备份混合数据代次")
		}
		if domain.Format != "nice-wiki.domain-snapshot" || (domain.SchemaVersion != 1 && domain.SchemaVersion != 2) || domain.BackupID != backupID || domain.BarrierID != barrierID || domain.SnapshotID != snapshot.SnapshotID || domain.DataSchemaVersion != snapshot.DataSchemaVersion || domain.RevisionWatermark != snapshot.RevisionWatermark || domain.ContainsSecrets != snapshot.ContainsSecrets {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "快照引用与已认证域清单不一致")
		}
		if domain.SchemaVersion==2&&(domain.Assets==nil||domain.Invocations==nil){return nil,nil,platform.NewError(platform.CodeIntegrityError,false,"V2域资产或调用完整库存未知")}
  if domain.SchemaVersion==1&&(len(domain.Assets)>0||len(domain.Invocations)>0){return nil,nil,platform.NewError(platform.CodeUnsupportedSchema,false,"V1不能混入新类型库存")}
  if domain.Files == nil || domain.Objects == nil || domain.Publications == nil || domain.References == nil {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "域权威枚举未知，不可用空值冒充无引用")
		}
		raw, err := platform.CanonicalJSON(domain)
		if err != nil {
			return nil, nil, err
		}
		if len(raw) > 16<<20 {
			return nil, nil, platform.NewError(platform.CodeBudgetExceeded, false, "域清单超过16MiB预算")
		}
		manifestHash := platform.HashBytes(raw)
		bindings = append(bindings, BackupDomainV1{Snapshot: snapshot, ManifestHash: manifestHash})
		for _, reference := range domain.References {
			bound := BackupReferenceV1{SourceOwner: domain.Owner, SourceManifestHash: manifestHash, Reference: reference}
			raw, err := platform.CanonicalJSON(bound)
			if err != nil {
				return nil, nil, err
			}
			key := string(raw)
			if seenRefs[key] {
				continue
			}
			seenRefs[key] = true
			referenceBytes += len(raw)
			if referenceBytes > maxBackupManifestBytes {
				return nil, nil, platform.NewError(platform.CodeBudgetExceeded, false, "全局引用索引超过2MiB预算")
			}
			references = append(references, bound)
		}
	}
	sort.Slice(references, func(i, j int) bool {
		a, _ := platform.CanonicalJSON(references[i])
		b, _ := platform.CanonicalJSON(references[j])
		return string(a) < string(b)
	})
	return bindings, references, nil
}

// validateManifestEvidence 以真实权威对象和发布修订查闭包，不以快照存在替代对象存在。
func validateManifestEvidence(manifest BackupManifestV1, domains []DomainSnapshotManifest) (ValidateCrossDomainRefsResult, error) {
 if manifest.SchemaVersion==2 {for _,domain:=range domains{if domain.SchemaVersion!=2{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"V2备份缺少拥有者新版完整库存，不能默补空")}}} else if manifest.SchemaVersion==1 {for _,bound:=range manifest.References{if bound.Reference.Kind!="object"&&bound.Reference.Kind!="publication"{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"历史V1备份混入新版引用")}}} else{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"备份清单版本不支持")}

	snapshots := make([]SnapshotRef, 0, len(manifest.Domains))
	for _, domain := range manifest.Domains {
		snapshots = append(snapshots, domain.Snapshot)
	}
	bindings, references, err := bindDomainEvidence(manifest.BackupID, manifest.SourceEpoch, manifest.BarrierID, snapshots, domains)
	if err != nil {
		return ValidateCrossDomainRefsResult{}, err
	}
	actual, _ := platform.CanonicalJSON(manifest.Domains)
	expected, _ := platform.CanonicalJSON(bindings)
	actualRefs, _ := platform.CanonicalJSON(manifest.References)
	expectedRefs, _ := platform.CanonicalJSON(references)
	if string(actual) != string(expected) || string(actualRefs) != string(expectedRefs) {
		return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "顶层清单并非域证据的完整确定性绑定")
	}
	return resolveReferenceTargets(domains, references)
}

func resolveReferenceTargets(domains []DomainSnapshotManifest, references []BackupReferenceV1) (ValidateCrossDomainRefsResult, error) {
	referenceRaw, _ := platform.CanonicalJSON(references)
	objects := map[string]string{}
	objectDefinitions := map[string]string{}
	publications := map[string]bool{}
	publicationIdentities := map[string]string{}
 assets:=map[string]string{};assetIdentities:=map[string]string{};invocations:=map[string]bool{};invocationIdentities:=map[string]string{}
	budget := len(referenceRaw)
	for _, domain := range domains {
		for _, object := range domain.Objects {
			ref := object.Reference
			if ref.Owner != domain.Owner {
				return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "对象由非拥有者冒充权威")
			}
			identity, _ := platform.CanonicalJSON(struct {
				Owner, ID string
				Version   int64
			}{ref.Owner, ref.ObjectID, ref.Version})
			raw, _ := platform.CanonicalJSON(ref)
			key := string(identity)
			if previous, found := objects[key]; found && previous != string(raw) {
				return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "同一对象版本声明不同摘要")
			}
			if _, found := objects[key]; !found {
				budget += len(raw)
				objects[key] = string(raw)
			}
			objectDefinitions[string(raw)] = string(raw)
		}
		if domain.Owner != "content" && len(domain.Publications) > 0 {
			return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布修订只能由内容域声明权威")
		}
		for _, publication := range domain.Publications {
			raw, _ := platform.CanonicalJSON(publication)
			identity, _ := platform.CanonicalJSON(struct {
				Document, Publication, Epoch string
				Revision                     int64
				Schema                       int
			}{publication.DocumentID, publication.PublishedRevisionID, publication.DataEpoch, publication.Revision, publication.SchemaVersion})
			if existing, found := publicationIdentities[string(identity)]; found && existing != string(raw) {
				return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "同一发布修订声明不同正文摘要")
			}
			publicationIdentities[string(identity)] = string(raw)
			key := string(raw)
			if !publications[key] {
				budget += len(raw)
				publications[key] = true
			}
		}
  if domain.Owner!="media"&&len(domain.Assets)>0{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"资产库存只能由媒体域声明")}
  for _,asset:=range domain.Assets{
   raw,_:=platform.CanonicalJSON(asset);key,_:=platform.CanonicalJSON(struct{ID string;Version int64}{asset.AssetID,asset.Version})
   if previous,ok:=assetIdentities[string(key)];ok&&previous!=string(raw){return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"同一资产版本声明不同原始字节")}
   if asset.AssetID==""||asset.Version<1||!platform.ValidHash(asset.SHA256)||asset.ByteSize<0{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"资产库存身份或原字节不完整")}
   assetIdentities[string(key)]=string(raw);if _,ok:=assets[string(raw)];!ok{budget+=len(raw);assets[string(raw)]=string(raw)}
  }
  if domain.Owner!="model-gateway"&&len(domain.Invocations)>0{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"调用库存只能由模型网关声明")}
  for _,invocation:=range domain.Invocations{
   if !platform.ValidIdentifier(invocation.InvocationID)||!platform.ValidIdentifier(invocation.RootOperationID)||invocation.DataEpoch!=domain.SourceEpoch{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"调用库存原根或代次不完整")}
   raw,_:=platform.CanonicalJSON(invocation)
   if previous,ok:=invocationIdentities[invocation.InvocationID];ok&&previous!=string(raw){return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"同一原调用归属声明矛盾")};invocationIdentities[invocation.InvocationID]=string(raw)
   if !invocations[string(raw)]{budget+=len(raw);invocations[string(raw)]=true}
  }
		if budget > maxBackupManifestBytes {
			return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "实际权威对象及引用索引超过2MiB预算")
		}
	}
	result := ValidateCrossDomainRefsResult{Valid: true, Issues: []gen.SafeIssue{}}
	for _, bound := range references {
		reference := bound.Reference
		found := false
		switch reference.Kind {
		case "object":
			if reference.Object == nil || reference.Publication != nil || reference.Asset!=nil || reference.Invocation!=nil {
				return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "对象引用分支不完整")
			}
			raw, _ := platform.CanonicalJSON(*reference.Object)
			_, found = objectDefinitions[string(raw)]
		case "publication":
			if reference.Publication == nil || reference.Object != nil || reference.Asset!=nil || reference.Invocation!=nil {
				return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布引用分支不完整")
			}
			raw, _ := platform.CanonicalJSON(*reference.Publication)
			found = publications[string(raw)]
  case "asset":
   if reference.Asset==nil||reference.Object!=nil||reference.Publication!=nil||reference.Invocation!=nil{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"资产引用分支不完整")}
   raw,_:=platform.CanonicalJSON(*reference.Asset);_,found=assets[string(raw)]
  case "invocation":
   if reference.Invocation==nil||reference.Object!=nil||reference.Publication!=nil||reference.Asset!=nil{return ValidateCrossDomainRefsResult{},platform.NewError(platform.CodeIntegrityError,false,"调用引用分支不完整")}
   raw,_:=platform.CanonicalJSON(*reference.Invocation);found=invocations[string(raw)]

		default:
			return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeIntegrityError, false, "跨域引用类型未知")
		}
		if !found {
			result.Valid = false
			if len(result.Issues) == 10000 {
				return ValidateCrossDomainRefsResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "跨域引用报告超过上限")
			}
			result.Issues = append(result.Issues, gen.SafeIssue{Code: gen.INTEGRITYERROR, Owner: bound.SourceOwner, Field: "references", Reason: "引用目标未出现在实际已认证权威清单中"})
		}
	}
	return result, nil
}

func BuildManifest(ctx context.Context, op OperationContext, in BuildManifestInput) (BuildManifestResult, error) {
 for _,domain:=range in.DomainManifests{if domain.SchemaVersion!=2{return BuildManifestResult{},platform.NewError(platform.CodeUnsupportedSchema,false,"新备份必须取得每个拥有者V2完整库存")}}
	domains, references, err := bindDomainEvidence(in.BackupId, in.SourceEpoch, in.BarrierId, in.Snapshots, in.DomainManifests)
	if err != nil {
		return BuildManifestResult{}, err
	}
	manifest := BackupManifestV1{Format: "nice-wiki.backup", SchemaVersion: 2, BackupID: in.BackupId, SourceEpoch: in.SourceEpoch, BarrierID: in.BarrierId, CreatedAt: in.CreatedAt, KeyID: in.KeyId, Domains: domains, References: references}
	validation, err := resolveReferenceTargets(in.DomainManifests, references)
	if err != nil {
		return BuildManifestResult{}, err
	}
	if !validation.Valid {
		return BuildManifestResult{}, platform.NewError(platform.CodeIntegrityError, false, "备份跨域引用未闭合")
	}
	raw, err := platform.CanonicalJSON(manifest)
	if err != nil {
		return BuildManifestResult{}, err
	}
	if len(raw) > maxBackupManifestBytes {
		return BuildManifestResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "顶层备份清单超过2MiB预算")
	}
	return BuildManifestResult{ManifestJSON: string(raw), ManifestHash: platform.HashBytes(raw)}, nil
}

func decodeBackupManifest(store *Store, raw string) (BackupManifestV1, error) {
	validate := store.PayloadValidators["BackupManifestV1"]
	if validate == nil {
		return BackupManifestV1{}, platform.NewError(platform.CodeDependencyUnavailable, false, "备份清单严格校验器未装配")
	}
	if len(raw) > maxBackupManifestBytes {
		return BackupManifestV1{}, platform.NewError(platform.CodeBudgetExceeded, false, "备份清单超过2MiB预算")
	}
	if validate(json.RawMessage(raw)) != nil {
		return BackupManifestV1{}, platform.NewError(platform.CodeIntegrityError, false, "备份清单不符合登记版本严格格式")
	}
	var manifest BackupManifestV1
	if json.Unmarshal([]byte(raw), &manifest) != nil {
		return BackupManifestV1{}, platform.NewError(platform.CodeIntegrityError, false, "备份清单无法解码")
	}
	canonical, err := platform.CanonicalJSON(manifest)
	if err != nil || string(canonical) != raw {
		return BackupManifestV1{}, platform.NewError(platform.CodeIntegrityError, false, "备份清单非完整canonical编码")
	}
	return manifest, nil
}

func ValidateCrossDomainRefs(ctx context.Context, op OperationContext, in ValidateCrossDomainRefsInput) (ValidateCrossDomainRefsResult, error) {
	store, err := Catalog(op)
	if err != nil {
		return ValidateCrossDomainRefsResult{}, err
	}
	manifest, err := decodeBackupManifest(store, in.ManifestJSON)
	if err != nil {
		return ValidateCrossDomainRefsResult{}, err
	}
	return validateManifestEvidence(manifest, in.DomainManifests)
}

// MarshalJSON保留V1历史原编码；新V2即使真实空库存也必须显式写[]。
func (manifest DomainSnapshotManifest) MarshalJSON()([]byte,error){
 type plain DomainSnapshotManifest
 raw,err:=json.Marshal(plain(manifest));if err!=nil{return nil,err}
 if manifest.SchemaVersion!=1{return raw,nil}
 if len(manifest.Assets)>0||len(manifest.Invocations)>0{return nil,platform.NewError(platform.CodeUnsupportedSchema,false,"V1域清单混入新类型库存")}
 var value map[string]json.RawMessage;if err=json.Unmarshal(raw,&value);err!=nil{return nil,err};delete(value,"assets");delete(value,"invocations");return json.Marshal(value)
}
