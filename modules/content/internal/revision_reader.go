package content

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/getkin/kin-openapi/openapi3"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/content/internal/gen"
)

type ReadRevisionBodyInput struct {
	DocumentID   Identifier `json:"documentId"`
	RevisionID   Identifier `json:"revisionId"`
	ExpectedHash *Hash      `json:"expectedHash"`
}
type ReadRevisionBodyResult struct {
	Body   DocumentBody  `json:"body"`
	Object gen.ObjectRef `json:"object"`
}

// PreparedRevisionFile将准备原结果与当前私有决定区分，不改变原8字段JSONL格式。
// preparing表示准备材料已持久、文件尚未确认；prepared/committed重放原结果。
type PreparedRevisionFile struct {
	Result WriteRevisionFileResult
	State  string
}

type CommittedRevisionFile struct {
	Draft   DraftRevision
	Journal CommitJournal
}

// RevisionFileStore扩展既有注入。适配器未实现这些持久操作时，文件用例拒绝运行。
// 所有键固定caller/actor/site/dataEpoch/operation，摘要不含原始正文。
// 准备不能推进head、改变ledger或发布；业务提交仍由DraftCommitTransaction负责。
type RevisionFileStore interface {
	DraftCommitStore
	RevisionDataEpoch() Identifier
	RevisionMaxDocumentBytes() int64
	RevisionWriteWindow() platform.WriteWindowConfig
	// Find合并私有决定/JSONL最新状态与COMMITTED receipt；已隔离/终止不复活。
	// 同键异摘要CONFLICT，COMMITTED优先，返回首次不可变原结果。
	FindRevisionPreparation(context.Context, OperationContext, Hash) (PreparedRevisionFile, bool, error)
	// Prepare原子持久preparing及原材料；并发同键返原结果，同revision归属冲突拒绝。
	PrepareRevision(context.Context, OperationContext, Hash, WriteRevisionFileResult) (PreparedRevisionFile, error)
	// Mark只在同scope/commitId内把preparing变prepared，终局不能降级。
	MarkRevisionPrepared(context.Context, OperationContext, Identifier) error
	// 只返回同site/doc/revision/epoch、未隔离且有COMMITTED资格的完整权威材料；
	// 方法内结束私有快照，不持事务跨后续文件IO。
	ReadCommittedRevision(context.Context, OperationContext, Identifier, Identifier) (CommittedRevisionFile, error)
	// 按持久决定COMMITTED优先隔离：已提交则关闭文档公开资格；
	// 未提交仅隔离该准备，不关闭另一个合法head。不改正文，不得假成功。
	QuarantineRevision(context.Context, OperationContext, Identifier, Identifier, string) error
}

func revisionFileStoreFromContext(ctx context.Context) (RevisionFileStore, error) {
	base, ok := draftCommitStoreFromContext(ctx)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "正文文件私有存储未装配")
	}
	store, ok := base.(RevisionFileStore)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "正文准备及读取存储尚未实现")
	}
	return store, nil
}

func revisionFileLocation(store RevisionFileStore, op OperationContext, documentID, revisionID Identifier) (string, string, error) {
	root, site, err := validateDraftCommitStore(store, op)
	if err != nil {
		return "", "", err
	}
	if store.RevisionDataEpoch() != op.DataEpoch {
		return "", "", platform.NewError(platform.CodeVersionMismatch, false, "正文文件存储代次已变化")
	}
	// Identifier允许点；OS路径组件不得把点解释成站点或目录跳转。
	if site == "." || site == ".." || documentID == "." || documentID == ".." {
		return "", "", platform.NewError(platform.CodeIntegrityError, false, "正文路径组件不合法")
	}
	if err := lifecycle.ValidatePrivatePath(root, "directory", false); err != nil {
		return "", "", err
	}
	path := revisionBodyPath(root, site, documentID, revisionID)
	if !pathWithin(filepath.Join(root, site), path) {
		return "", "", platform.NewError(platform.CodeIntegrityError, false, "正文路径越过站点范围")
	}
	return root, path, nil
}

// ReadRevisionBody从可信提交索引读精确COMMITTED文件，不按当前head猜历史哈希。
func ReadRevisionBody(ctx context.Context, op OperationContext, input ReadRevisionBodyInput) (ReadRevisionBodyResult, error) {
	store, err := revisionFileStoreFromContext(ctx)
	if err != nil {
		return ReadRevisionBodyResult{}, err
	}
	_, path, err := revisionFileLocation(store, op, input.DocumentID, input.RevisionID)
	if err != nil {
		return ReadRevisionBodyResult{}, err
	}
	record, err := store.ReadCommittedRevision(ctx, op, input.DocumentID, input.RevisionID)
	if err != nil {
		return ReadRevisionBodyResult{}, err
	}
	if err := validateCommittedMaterial(input, record); err != nil {
		return ReadRevisionBodyResult{}, isolateRevisionRead(ctx, op, store, input, err)
	}
	if input.ExpectedHash != nil && *input.ExpectedHash != record.Draft.BodyHash {
		// 调用方的错误引用不能隔离一个与私有权威一致的文件。
		return ReadRevisionBodyResult{}, platform.NewError(platform.CodeIntegrityError, false, "请求正文哈希与已提交引用不一致")
	}
	raw, err := readImmutableRevision(ctx, path, record.Draft.Object)
	if err != nil {
		return ReadRevisionBodyResult{}, isolateRevisionRead(ctx, op, store, input, err)
	}
	body, err := decodeStoredBody(raw)
	if err != nil {
		return ReadRevisionBodyResult{}, isolateRevisionRead(ctx, op, store, input, err)
	}
	if _, err := ValidateBlockTree(ctx, op, ValidateBlockTreeInput{Body: body, DocumentID: input.DocumentID}); err != nil {
		return ReadRevisionBodyResult{}, isolateRevisionRead(ctx, op, store, input, platform.NewError(platform.CodeIntegrityError, false, "已提交正文语义不合法"))
	}
	canonical, err := CanonicalizeBody(CanonicalizeBodyInput{Body: body})
	if err != nil || canonical.ContentHash != record.Draft.BodyHash {
		return ReadRevisionBodyResult{}, isolateRevisionRead(ctx, op, store, input, platform.NewError(platform.CodeIntegrityError, false, "已提交正文不是规范编码"))
	}
	current, err := store.ReadCommittedRevision(ctx, op, input.DocumentID, input.RevisionID)
	if err != nil {
		return ReadRevisionBodyResult{}, err
	}
	if current.Draft.Object != record.Draft.Object || !sameJournalIdentity(current.Journal, record.Journal) || current.Journal.State != "committed" {
		return ReadRevisionBodyResult{}, isolateRevisionRead(ctx, op, store, input, platform.NewError(platform.CodeIntegrityError, false, "正文读取期间提交资格发生矛盾"))
	}
	return ReadRevisionBodyResult{Body: body, Object: record.Draft.Object}, nil
}

func validateCommittedMaterial(input ReadRevisionBodyInput, record CommittedRevisionFile) error {
	draft, journal := record.Draft, record.Journal
	if !validCommitJournal(journal) || journal.State != "committed" || draft.DocumentId != input.DocumentID || draft.RevisionId != input.RevisionID ||
		journal.DocumentID != input.DocumentID || journal.RevisionID != input.RevisionID || journal.BodyHash != draft.BodyHash ||
		draft.DraftVersion < 1 || !platform.ValidIdentifier(draft.AuthorAccountId) || Timestamp(draft.CreatedAt.UTC().Format(time.RFC3339Nano)) != journal.CreatedAt || !sameIdentifierPointer(draft.PreviousRevisionId, journal.ExpectedHead) ||
		draft.Object.Owner != "content" || draft.Object.ObjectId != input.RevisionID || draft.Object.Version != 1 || draft.Object.Sha256 != draft.BodyHash ||
		draft.Object.MediaType != "application/json" || draft.Object.ByteSize < 0 || draft.Object.ByteSize > maxBodyBytes {
		return platform.NewError(platform.CodeIntegrityError, false, "已提交正文引用或归属不一致")
	}
	return nil
}

func readImmutableRevision(ctx context.Context, path string, object gen.ObjectRef) ([]byte, error) {
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", false); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "已提交正文不存在")
	}
	if info.Size() != int64(object.ByteSize) || info.Size() > maxBodyBytes {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "正文文件大小不一致")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "正文文件不可读")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "正文文件身份已变化")
	}
	raw, err := io.ReadAll(io.LimitReader(revisionContextReader{ctx: ctx, reader: file}, maxBodyBytes+1))
	if err != nil {
		return nil, normalizeDraftCommitError(err, "正文文件读取失败")
	}
	if len(raw) != object.ByteSize || len(raw) > maxBodyBytes || platform.HashBytes(raw) != object.Sha256 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "正文文件长度或哈希不一致")
	}
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", false); err != nil {
		return nil, err
	}
	return raw, nil
}

type revisionContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader revisionContextReader) Read(buffer []byte) (int, error) {
	if err := checkContextOnly(reader.ctx); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

var storedBodySchemaOnce sync.Once
var storedBodySchema *openapi3.Schema
var storedBodySchemaError error

// decodeStoredBody是持久文件边界：在此一次严格验证源schema，拒未知字段/版本。
func decodeStoredBody(raw []byte) (DocumentBody, error) {
	if !utf8.Valid(raw) {
		return DocumentBody{}, platform.NewError(platform.CodeIntegrityError, false, "正文文件不是UTF-8")
	}
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return DocumentBody{}, platform.NewError(platform.CodeIntegrityError, false, "正文文件JSON不合法")
	}
	if value["schemaVersion"] != float64(1) {
		return DocumentBody{}, platform.NewError(platform.CodeUnsupportedSchema, false, "正文格式版本不受支持")
	}
	storedBodySchemaOnce.Do(func() {
		spec, err := gen.GetSwagger()
		if err != nil {
			storedBodySchemaError = err
			return
		}
		ref := spec.Components.Schemas["DocumentBody"]
		if ref == nil || ref.Value == nil {
			storedBodySchemaError = errors.New("正文schema缺失")
			return
		}
		storedBodySchema = ref.Value
	})
	if storedBodySchemaError != nil {
		return DocumentBody{}, platform.NewError(platform.CodeDependencyUnavailable, true, "正文schema尚不可用")
	}
	if err := storedBodySchema.VisitJSON(value); err != nil {
		return DocumentBody{}, platform.NewError(platform.CodeIntegrityError, false, "正文文件不符合严格schema")
	}
	var body DocumentBody
	if json.Unmarshal(raw, &body) != nil {
		return DocumentBody{}, platform.NewError(platform.CodeIntegrityError, false, "正文文件不可解码")
	}
	return body, nil
}

func isolateRevisionRead(ctx context.Context, op OperationContext, store RevisionFileStore, input ReadRevisionBodyInput, cause error) error {
	var failure *platform.Error
	if !errors.As(cause, &failure) || (failure.Code != platform.CodeIntegrityError && failure.Code != platform.CodeUnsupportedSchema) {
		return cause
	}
	if err := store.QuarantineRevision(ctx, op, input.DocumentID, input.RevisionID, "body_integrity"); err != nil {
		return normalizeDraftCommitError(err, "正文损坏隔离决定不可持久化")
	}
	return cause
}
