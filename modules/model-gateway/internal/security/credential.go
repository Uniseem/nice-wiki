// Package security保存本域凭据密文，不连接供应方或记录请求正文。
package security

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
 "strconv"
	"syscall"
	"time"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)

type OperationContext = platform.OperationContext
type PutCredentialInput = gen.PutCredentialInput
type PutCredentialResult = gen.PutCredentialResult
type RotateCredentialInput = gen.RotateCredentialInput
type RotateCredentialResult = gen.RotateCredentialResult

type credentialCommand struct {
	ProviderID      string
	Secret          string
	ExpectedVersion int
	IdempotencyKey  string
	Rotate          bool
}

type credentialWriteResult struct {
	Version        int
	RetiredVersion int
	Fingerprint    string
}

// PutCredential保存首个或替换当前凭据；任何旧密文均保留。
func PutCredential(ctx context.Context, op OperationContext, input PutCredentialInput) (PutCredentialResult, error) {
	result, err := writeCredential(ctx, op, credentialCommand{input.ProviderId, input.Secret, input.ExpectedVersion, input.IdempotencyKey, false})
	if err != nil {
		return PutCredentialResult{}, err
	}
	return PutCredentialResult{CredentialVersion: result.Version, Fingerprint: result.Fingerprint}, nil
}

// RotateCredential切换新版本；只有全部真实原引用释放后才销毁旧密文并保留审计。
func RotateCredential(ctx context.Context, op OperationContext, input RotateCredentialInput) (RotateCredentialResult, error) {
	result, err := writeCredential(ctx, op, credentialCommand{input.ProviderId, input.NewSecret, input.ExpectedVersion, input.IdempotencyKey, true})
	if err != nil {
		return RotateCredentialResult{}, err
	}
	return RotateCredentialResult{CredentialVersion: result.Version, RetiredVersion: result.RetiredVersion}, nil
}

func writeCredential(ctx context.Context, op OperationContext, input credentialCommand) (credentialWriteResult, error) {
	if op.Actor.Kind != "admin" || !slices.Contains(op.Actor.Grants, "superadmin") {
		return credentialWriteResult{}, platform.NewError(platform.CodeForbidden, false, "仅超管可修改供应方凭据")
	}
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据存储尚未装配")
	}
	masterKey, err := loadCredentialMasterKey(store.MasterKeyRef())
	if err != nil {
		return credentialWriteResult{}, err
	}
	defer clear(masterKey)
	functionID := "model-gateway.PutCredential"
	if input.Rotate {
		functionID = "model-gateway.RotateCredential"
	}
	digest, err := credentialDigest(masterKey, "receipt", struct {
		CallerService   string `json:"callerService"`
		DataEpoch       string `json:"dataEpoch"`
		FunctionID      string `json:"functionId"`
		ProviderID      string `json:"providerId"`
		ExpectedVersion int    `json:"expectedVersion"`
		Secret          string `json:"secret"`
	}{op.CallerService, op.DataEpoch, functionID, input.ProviderID, input.ExpectedVersion, input.Secret})
	if err != nil {
		return credentialWriteResult{}, err
	}
	result := credentialWriteResult{Version: input.ExpectedVersion + 1, RetiredVersion: input.ExpectedVersion}
	var receiptResult any
	if input.Rotate {
		receiptResult = RotateCredentialResult{CredentialVersion: result.Version, RetiredVersion: result.RetiredVersion}
	} else {
		result.Fingerprint, err = credentialFingerprint(masterKey, input.ProviderID, input.Secret)
		if err != nil {
			return credentialWriteResult{}, err
		}
		receiptResult = PutCredentialResult{CredentialVersion: result.Version, Fingerprint: result.Fingerprint}
	}
	resultJSON, err := json.Marshal(receiptResult)
	if err != nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeInternal, false, "凭据脱敏结果编码失败")
	}
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	defer tx.Rollback()
	var epoch string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton=1`).Scan(&epoch); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if epoch == "" {
		return credentialWriteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据当前代次尚未知")
	}
	if epoch != op.DataEpoch {
		return credentialWriteResult{}, platform.NewError(platform.CodeVersionMismatch, false, "凭据当前代次已变化")
	}
	if found, err := readCredentialReceipt(ctx, op, tx, functionID, input.IdempotencyKey, digest, resultJSON); err != nil {
		return credentialWriteResult{}, err
	} else if found {
		return result, nil
	}
	var blocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM barriers WHERE state<>'released')`).Scan(&blocked); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if blocked {
		return credentialWriteResult{}, platform.NewError(platform.CodeLocked, false, "凭据配置屏障尚未释放")
	}
	var providerVersion, currentVersion int
	var providerEpoch string
	var enabled bool
	err = tx.QueryRowContext(ctx, `SELECT version,credential_version,data_epoch,enabled FROM providers WHERE provider_id=?`, input.ProviderID).Scan(&providerVersion, &currentVersion, &providerEpoch, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return credentialWriteResult{}, platform.NewError(platform.CodeNotFound, false, "凭据供应方不存在")
	}
	if err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if providerEpoch != op.DataEpoch || currentVersion != input.ExpectedVersion {
		return credentialWriteResult{}, platform.NewError(platform.CodeVersionMismatch, false, "供应方凭据版本或代次已变化")
	}
	if providerVersion >= 9007199254740991 {
		return credentialWriteResult{}, platform.NewError(platform.CodeVersionMismatch, false, "供应方版本已达到上限")
	}
	if input.Rotate {
		var oldEpoch string
		var nonceSize, ciphertextSize int
		err := tx.QueryRowContext(ctx, `SELECT data_epoch,length(nonce),length(ciphertext) FROM credentials WHERE provider_id=? AND credential_version=?`, input.ProviderID, currentVersion).Scan(&oldEpoch, &nonceSize, &ciphertextSize)
		if errors.Is(err, sql.ErrNoRows) {
			return credentialWriteResult{}, platform.NewError(platform.CodeIntegrityError, false, "当前凭据材料缺失，不能轮转")
		}
		if err != nil {
			return credentialWriteResult{}, credentialStorageError(err)
		}
		if oldEpoch != op.DataEpoch {
			return credentialWriteResult{}, platform.NewError(platform.CodeVersionMismatch, false, "当前凭据代次已变化")
		}
		if nonceSize != 12 || ciphertextSize <= 16 {redacted,e:=gatewayruntime.IsRedactedMaterial(ctx,tx,"credential",input.ProviderID+"|"+strconv.Itoa(currentVersion));if e!=nil{return credentialWriteResult{},e};if !redacted||nonceSize!=0||ciphertextSize!=0{return credentialWriteResult{},platform.NewError(platform.CodeIntegrityError,false,"当前凭据材料不完整")}}
	}
	var reused bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM credentials WHERE provider_id=? AND credential_version=?) OR EXISTS(SELECT 1 FROM credential_retirements WHERE provider_id=? AND credential_version=?)`, input.ProviderID, result.Version, input.ProviderID, result.Version).Scan(&reused); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if reused {
		return credentialWriteResult{}, platform.NewError(platform.CodeIntegrityError, false, "新凭据版本已存在历史材料")
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeInternal, false, "凭据加密初始化失败")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeInternal, false, "凭据加密初始化失败")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据随机材料不可用")
	}
	aad, err := platform.CanonicalJSON(struct {
		Format            string `json:"format"`
		ProviderID        string `json:"providerId"`
		CredentialVersion int    `json:"credentialVersion"`
	}{"credential-aes256gcm-v1", input.ProviderID, result.Version})
	if err != nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeInternal, false, "凭据认证材料编码失败")
	}
	plaintext := []byte(input.Secret)
	defer clear(plaintext)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	if _, err := tx.ExecContext(ctx, `INSERT INTO credentials(provider_id,credential_version,nonce,ciphertext,data_epoch) VALUES(?,?,?,?,?)`, input.ProviderID, result.Version, nonce, ciphertext, op.DataEpoch); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	now := platform.Now()
	updated, err := tx.ExecContext(ctx, `UPDATE providers SET credential_version=?,version=version+1,updated_at=? WHERE provider_id=? AND version=? AND credential_version=? AND data_epoch=?`, result.Version, now, input.ProviderID, providerVersion, input.ExpectedVersion, op.DataEpoch)
	if err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if affected != 1 {
		return credentialWriteResult{}, platform.NewError(platform.CodeVersionMismatch, false, "供应方或凭据版本已变化")
	}
	payload, err := json.Marshal(struct {
		ProviderID string `json:"providerId"`
		Version    int    `json:"version"`
		Enabled    bool   `json:"enabled"`
		DataEpoch  string `json:"dataEpoch"`
	}{input.ProviderID, providerVersion + 1, enabled, op.DataEpoch})
	if err != nil {
		return credentialWriteResult{}, platform.NewError(platform.CodeInternal, false, "供应方凭据事件编码失败")
	}
	if err := outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "provider.changed", EventVersion: 1, Producer: "model-gateway", SubjectID: input.ProviderID, Revision: platform.Revision(providerVersion + 1), OccurredAt: now, DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
		return credentialWriteResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM operation_receipts WHERE site_id=? AND actor_id=? AND function_id=? AND idempotency_key=?`, op.Actor.SiteID, op.Actor.SubjectID, functionID, input.IdempotencyKey); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO operation_receipts(site_id,actor_id,function_id,idempotency_key,input_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?)`, op.Actor.SiteID, op.Actor.SubjectID, functionID, input.IdempotencyKey, digest, resultJSON, now); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	if err := gatewayruntime.DestroyRetiredSecrets(ctx, op, tx); err != nil {
		return credentialWriteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return credentialWriteResult{}, credentialStorageError(err)
	}
	return result, nil
}

func loadCredentialMasterKey(ref string) ([]byte, error) {
	if ref == "" || !filepath.IsAbs(ref) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据主密钥引用尚未装配")
	}
	before, err := os.Lstat(ref)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据主密钥不可读取")
	}
	if err := lifecycle.ValidatePrivatePath(ref, "regular-file", false); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(ref, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据主密钥不可打开")
	}
	file := os.NewFile(uintptr(fd), ref)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "凭据主密钥打开期间身份变化")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 33))
	valid := false
	defer func() {
		if !valid {
			clear(raw)
		}
	}()
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "凭据主密钥不可读取")
	}
	if len(raw) != 32 {
		return nil, platform.NewError(platform.CodeValidation, false, "凭据主密钥必须恰好32字节")
	}
	if err := lifecycle.ValidatePrivatePath(ref, "regular-file", false); err != nil {
		return nil, err
	}
	current, err := os.Lstat(ref)
	if err != nil || !os.SameFile(current, opened) || current.Size() != 32 || !current.ModTime().Equal(opened.ModTime()) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "凭据主密钥路径或内容身份变化")
	}
	valid = true
	return raw, nil
}

func credentialDigest(key []byte, purpose string, value any) (string, error) {
	raw, err := platform.CanonicalJSON(value)
	if err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "凭据敏感摘要编码失败")
	}
	defer clear(raw)
	derive := hmac.New(sha256.New, key)
	_, _ = derive.Write([]byte("nice-wiki.model-gateway.credential." + purpose + ".v1"))
	subkey := derive.Sum(nil)
	defer clear(subkey)
	mac := hmac.New(sha256.New, subkey)
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func credentialFingerprint(key []byte, providerID, secret string) (string, error) {
	digest, err := credentialDigest(key, "fingerprint", struct {
		ProviderID string `json:"providerId"`
		Secret     string `json:"secret"`
	}{providerID, secret})
	if err != nil {
		return "", err
	}
	suffix := ""
	if utf8.RuneCountInString(secret) > 8 {
		start := len(secret)
		for range 4 {
			_, size := utf8.DecodeLastRuneInString(secret[:start])
			start -= size
		}
		suffix = secret[start:]
	}
	return "****" + suffix + "#" + digest[:12], nil
}

func readCredentialReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, functionID, key, digest string, expectedJSON []byte) (bool, error) {
	var storedHash, createdAt string
	var resultJSON []byte
	err := tx.QueryRowContext(ctx, `SELECT input_hash,result_json,created_at FROM operation_receipts WHERE site_id=? AND actor_id=? AND function_id=? AND idempotency_key=?`, op.Actor.SiteID, op.Actor.SubjectID, functionID, key).Scan(&storedHash, &resultJSON, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, credentialStorageError(err)
	}
	created, err := platform.ParseTimestamp(createdAt)
	now := time.Now().UTC()
	if err != nil || created.After(now) {
		return false, platform.NewError(platform.CodeIntegrityError, false, "凭据回执时间损坏")
	}
	if now.Sub(created) > 7*24*time.Hour {
		return false, nil
	}
	if !hmac.Equal([]byte(storedHash), []byte(digest)) {
		return false, platform.NewError(platform.CodeConflict, false, "凭据幂等输入或部署主密钥协议不一致")
	}
	if !bytes.Equal(resultJSON, expectedJSON) {
		return false, platform.NewError(platform.CodeIntegrityError, false, "凭据脱敏原结果回执损坏")
	}
	return true, nil
}

func credentialStorageError(err error) error {
	var public *platform.Error
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, false, "凭据存储操作已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, false, "凭据存储操作已到截止")
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "凭据权威存储暂不可用")
}

// NewCredentialHandlers只做真实凭据写入绑定，授权与严格schema由公共入口完成。
func NewCredentialHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"putCredential": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var input PutCredentialInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "凭据输入不可读"), op.RequestID))
				return
			}
			if r.PathValue("providerId") != input.ProviderId || (r.Header.Get("Idempotency-Key") != "" && r.Header.Get("Idempotency-Key") != input.IdempotencyKey) {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "凭据路径或幂等头与正文不一致"), op.RequestID))
				return
			}
			result, err := PutCredential(r.Context(), op, input)
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(result)
		}),
		"rotateCredential": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, ok := platform.OperationContextFrom(r.Context())
			if !ok {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeUnauthenticated, false, "缺少可信服务操作上下文"), platform.NewIdentifier()))
				return
			}
			var input RotateCredentialInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "凭据输入不可读"), op.RequestID))
				return
			}
			if r.PathValue("providerId") != input.ProviderId || (r.Header.Get("Idempotency-Key") != "" && r.Header.Get("Idempotency-Key") != input.IdempotencyKey) {
				_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeValidation, false, "凭据路径或幂等头与正文不一致"), op.RequestID))
				return
			}
			result, err := RotateCredential(r.Context(), op, input)
			if err != nil {
				_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_ = json.NewEncoder(w).Encode(result)
		}),
	}
}
