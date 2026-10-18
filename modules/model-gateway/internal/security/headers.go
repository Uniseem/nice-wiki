package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"net/http"
	"nice-wiki/libs/platform"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"strings"
	"unicode/utf8"
)

type SecretHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type SealedHeader struct {
	RefID             string
	Name              string
	Nonce, Ciphertext []byte
}

// SecretHeaderName只允许应用请求头，不允许覆盖路由、cookie或主认证。
func SecretHeaderName(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "host", "content-length", "transfer-encoding", "cookie", "set-cookie", "authorization", "proxy-authorization", "connection", "upgrade", "te", "trailer", "content-type", "accept", "x-api-key", "x-goog-api-key", "anthropic-version", "forwarded", "x-forwarded-host", "x-forwarded-for", "x-forwarded-proto", "x-real-ip", "range", "expect", "accept-encoding", "content-encoding", "idempotency-key", "x-request-id":
		return false
	}
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}
func PrepareHeaderSecrets(ctx context.Context, op OperationContext, providerID string, providerVersion int, headers []SecretHeader, command any) ([]SealedHeader, string, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return nil, "", platform.NewError(platform.CodeDependencyUnavailable, false, "额外头私有存储未装配")
	}
	key, err := loadCredentialMasterKey(store.MasterKeyRef())
	if err != nil {
		return nil, "", err
	}
	defer clear(key)
	digest, err := credentialDigest(key, "provider-secret-headers", command)
	if err != nil {
		return nil, "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", platform.NewError(platform.CodeInternal, false, "额外头加密初始化失败")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", platform.NewError(platform.CodeInternal, false, "额外头加密初始化失败")
	}
	result := make([]SealedHeader, 0, len(headers))
	seen := make(map[string]bool)
	for _, header := range headers {
		lower := strings.ToLower(header.Name)
		if !SecretHeaderName(header.Name) || seen[lower] || header.Value == "" || len(header.Value) > 4096 || !utf8.ValidString(header.Value) || strings.ContainsAny(header.Value, "\r\n\x00") {
			return nil, "", platform.NewError(platform.CodeValidation, false, "额外请求头名称或秘密值不合法")
		}
		seen[lower] = true
		ref := platform.NewIdentifier()
		nonce := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return nil, "", platform.NewError(platform.CodeDependencyUnavailable, false, "额外头随机材料不可用")
		}
		aad, _ := platform.CanonicalJSON(struct {
			Format, ProviderID string
			ProviderVersion    int
			Name, Ref, Epoch   string
		}{"provider-header-aes256gcm-v1", providerID, providerVersion, http.CanonicalHeaderKey(header.Name), ref, op.DataEpoch})
		plaintext := []byte(header.Value)
		sealed := gcm.Seal(nil, nonce, plaintext, aad)
		clear(plaintext)
		result = append(result, SealedHeader{RefID: ref, Name: http.CanonicalHeaderKey(header.Name), Nonce: nonce, Ciphertext: sealed})
	}
	return result, digest, nil
}
func SaveHeaderSecrets(ctx context.Context, op OperationContext, tx *sql.Tx, providerID string, version int, headers []SealedHeader) error {
	for _, header := range headers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO header_secrets(ref_id,provider_id,provider_version,header_name,data_epoch,nonce,ciphertext) VALUES(?,?,?,?,?,?,?)`, header.RefID, providerID, version, header.Name, op.DataEpoch, header.Nonce, header.Ciphertext); err != nil {
			return credentialStorageError(err)
		}
	}
	return nil
}

// OpenAttemptCredential只能解密已固定路线真实引用的原版本，不追随active指针。
func OpenAttemptCredential(ctx context.Context, op OperationContext, attemptID string) ([]byte, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "原凭据私有存储未装配")
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return nil, err
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, attemptID)
	if err != nil {
		tx.Close()
		return nil, err
	}
	if _, err := gatewayruntime.ReadInvocationRecord(tx, op, attempt.Attempt.InvocationId); err != nil {
		tx.Close()
		return nil, err
	}
	var epoch string
	var nonce, ciphertext []byte
	err = tx.Tx.QueryRowContext(tx.Context, `SELECT c.data_epoch,c.nonce,c.ciphertext FROM credentials c JOIN route_credential_refs r ON r.provider_id=c.provider_id AND r.credential_version=c.credential_version AND r.data_epoch=c.data_epoch WHERE r.invocation_id=? AND r.provider_id=? AND r.credential_version=? AND r.released=0`, attempt.Attempt.InvocationId, attempt.Provider.ProviderId, attempt.CredentialVersion).Scan(&epoch, &nonce, &ciphertext)
	tx.Close()
	if err != nil {
		return nil, credentialStorageError(err)
	}
	if epoch != op.DataEpoch {
		return nil, platform.NewError(platform.CodeStaleResult, false, "原凭据引用代际已变化")
	}
	key, err := loadCredentialMasterKey(store.MasterKeyRef())
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "原凭据加密材料损坏")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "原凭据nonce损坏")
	}
	aad, _ := platform.CanonicalJSON(struct {
		Format            string `json:"format"`
		ProviderID        string `json:"providerId"`
		CredentialVersion int    `json:"credentialVersion"`
	}{"credential-aes256gcm-v1", attempt.Provider.ProviderId, attempt.CredentialVersion})
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "原凭据认证失败")
	}
	return plaintext, nil
}
func OpenAttemptHeaders(ctx context.Context, op OperationContext, attemptID string) (http.Header, error) {
	store := gatewayruntime.CurrentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "原额外头私有存储未装配")
	}
	tx, err := gatewayruntime.BeginExecution(ctx, op, false)
	if err != nil {
		return nil, err
	}
	attempt, err := gatewayruntime.ReadAttemptRecord(tx, attemptID)
	if err != nil {
		tx.Close()
		return nil, err
	}
	if _, err := gatewayruntime.ReadInvocationRecord(tx, op, attempt.Attempt.InvocationId); err != nil {
		tx.Close()
		return nil, err
	}
	type material struct {
		Name, Ref, Epoch  string
		Version           int
		Nonce, Ciphertext []byte
	}
	materials := make([]material, 0, len(attempt.Provider.ExtraHeaderSecretRefs))
	for index, ref := range attempt.Provider.ExtraHeaderSecretRefs {
		var stored material
		stored.Ref = ref
		err := tx.Tx.QueryRowContext(tx.Context, `SELECT h.header_name,h.data_epoch,h.provider_version,h.nonce,h.ciphertext FROM header_secrets h JOIN route_header_refs r ON r.ref_id=h.ref_id WHERE r.invocation_id=? AND r.ref_id=? AND r.released=0 AND h.provider_id=?`, attempt.Attempt.InvocationId, ref, attempt.Provider.ProviderId).Scan(&stored.Name, &stored.Epoch, &stored.Version, &stored.Nonce, &stored.Ciphertext)
		if err != nil {
			tx.Close()
			return nil, credentialStorageError(err)
		}
		projected,e:=gatewayruntime.IsRestoredEpoch(ctx,tx.Tx,stored.Epoch,op.DataEpoch);if e!=nil{tx.Close();return nil,e}
		if !projected || index >= len(attempt.Provider.ExtraHeaderNames) || stored.Name != attempt.Provider.ExtraHeaderNames[index] || !SecretHeaderName(stored.Name) {
			tx.Close()
			return nil, platform.NewError(platform.CodeIntegrityError, false, "额外头引用不属于原渠道或版本")
		}
		materials = append(materials, stored)
	}
	tx.Close()
	result := make(http.Header)
	if len(materials) == 0 {
		return result, nil
	}
	key, err := loadCredentialMasterKey(store.MasterKeyRef())
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "额外头加密材料损坏")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "额外头加密材料损坏")
	}
	for _, stored := range materials {
		if len(stored.Nonce) != gcm.NonceSize() {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "额外头nonce损坏")
		}
		aad, _ := platform.CanonicalJSON(struct {
			Format, ProviderID string
			ProviderVersion    int
			Name, Ref, Epoch   string
		}{"provider-header-aes256gcm-v1", attempt.Provider.ProviderId, stored.Version, stored.Name, stored.Ref, stored.Epoch})
		value, err := gcm.Open(nil, stored.Nonce, stored.Ciphertext, aad)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "额外头认证失败")
		}
		if strings.ContainsAny(string(value), "\r\n\x00") {
			clear(value)
			return nil, platform.NewError(platform.CodeIntegrityError, false, "额外头秘密包含非法控制字符")
		}
		result.Set(stored.Name, string(value))
		clear(value)
	}
	return result, nil
}
