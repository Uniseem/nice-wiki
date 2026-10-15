package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

// Password 与 types.json 的敏感口令标量对应；本域不记录其原值。
type Password = string

// OperationContext 是服务间可信操作上下文的本域别名。
type OperationContext = platform.OperationContext

// HashPasswordInput 是生成口令哈希的输入。
type HashPasswordInput struct {
	Password Password `json:"password"`
}

// HashPasswordResult 是生成口令哈希的结果。
type HashPasswordResult struct {
	EncodedHash string `json:"encodedHash"`
}

// VerifyPasswordInput 是核对口令的输入。
type VerifyPasswordInput struct {
	Password    string `json:"password"`
	EncodedHash string `json:"encodedHash"`
}

// VerifyPasswordResult 是核对口令的结果。
type VerifyPasswordResult struct {
	Valid       bool `json:"valid"`
	NeedsRehash bool `json:"needsRehash"`
}

// ValidatePasswordChangeInput 是改密三输入规则的输入。
type ValidatePasswordChangeInput struct {
	OldPassword  string   `json:"oldPassword"`
	NewPassword  Password `json:"newPassword"`
	Confirmation Password `json:"confirmation"`
}

// ValidatePasswordChangeResult 是改密三输入规则的结果。
type ValidatePasswordChangeResult struct {
	Valid bool `json:"valid"`
}

const (
	argonMemoryKiB      = uint32(65536)
	argonTime           = uint32(3)
	argonParallelism    = uint8(4)
	argonHashBytes      = uint32(32)
	argonSaltBytes      = 16
	argonMinSaltBytes   = 8
	argonMaxSaltBytes   = 64
	argonMaxHashBytes   = 64
	passwordMaxRunes    = 256
	passwordMinRunes    = 8
	verifyMinRunes      = 1
	passwordConcurrency = 4
)

// Argon2id 需要有界并发，避免多个登录/改密请求同时消耗过多内存。
var passwordSlots = make(chan struct{}, passwordConcurrency)

// HashPassword 使用合同固定参数生成 PHC Argon2id v19 编码哈希。
func HashPassword(ctx context.Context, op OperationContext, input HashPasswordInput) (HashPasswordResult, error) {
	if !validPassword(input.Password) {
		return HashPasswordResult{}, platform.NewError(platform.CodeValidation, false, "口令不合法")
	}

	workCtx, cancel, err := credentialContext(ctx, op)
	if err != nil {
		return HashPasswordResult{}, err
	}
	defer cancel()
	if err := acquirePasswordSlot(workCtx); err != nil {
		return HashPasswordResult{}, err
	}
	defer func() { <-passwordSlots }()

	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return HashPasswordResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "随机源不可用")
	}
	defer clear(salt)

	passwordBytes := []byte(input.Password)
	defer clear(passwordBytes)
	derived := argon2.IDKey(passwordBytes, salt, argonTime, argonMemoryKiB, argonParallelism, argonHashBytes)
	if err := credentialContextError(workCtx.Err()); err != nil {
		clear(derived)
		return HashPasswordResult{}, err
	}
	encoded := encodeArgonHash(salt, derived)
	clear(derived)
	return HashPasswordResult{EncodedHash: encoded}, nil
}

// VerifyPassword 按 PHC 编码派生并以常量时间核对口令；超出当前成本上限
// 的编码或损坏编码一律隔离为完整性错误，不执行降级算法。
func VerifyPassword(input VerifyPasswordInput) (VerifyPasswordResult, error) {
	if !validVerifyPassword(input.Password) || len(input.EncodedHash) == 0 || len(input.EncodedHash) > 512 || !utf8.ValidString(input.EncodedHash) {
		return VerifyPasswordResult{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希不合法")
	}
	parsed, err := parseArgonHash(input.EncodedHash)
	if err != nil {
		return VerifyPasswordResult{}, err
	}

	if err := acquirePasswordSlot(context.Background()); err != nil {
		return VerifyPasswordResult{}, err
	}
	defer func() { <-passwordSlots }()

	passwordBytes := []byte(input.Password)
	defer clear(passwordBytes)
	expected := argon2.IDKey(passwordBytes, parsed.salt, parsed.time, parsed.memory, parsed.parallelism, uint32(len(parsed.digest)))
	defer clear(expected)
	valid := subtle.ConstantTimeCompare(expected, parsed.digest) == 1
	return VerifyPasswordResult{
		Valid:       valid,
		NeedsRehash: valid && (parsed.memory != argonMemoryKiB || parsed.time != argonTime || parsed.parallelism != argonParallelism || uint32(len(parsed.digest)) != argonHashBytes),
	}, nil
}

// ValidatePasswordChange 校验改密的长度、两次新口令一致且不同于旧口令。
// 语义不满足时返回 valid=false；输入超出 schema 时返回 VALIDATION，且不回显口令。
func ValidatePasswordChange(input ValidatePasswordChangeInput) (ValidatePasswordChangeResult, error) {
	if !validNonEmptyPassword(input.OldPassword) || !validPassword(input.NewPassword) || !validPassword(input.Confirmation) {
		return ValidatePasswordChangeResult{}, platform.NewError(platform.CodeValidation, false, "口令不合法")
	}
	if subtle.ConstantTimeCompare([]byte(input.NewPassword), []byte(input.Confirmation)) != 1 {
		return ValidatePasswordChangeResult{Valid: false}, nil
	}
	if !passwordEqual(input.OldPassword, input.NewPassword) {
		return ValidatePasswordChangeResult{Valid: true}, nil
	}
	return ValidatePasswordChangeResult{Valid: false}, nil
}

type parsedArgonHash struct {
	memory      uint32
	time        uint32
	parallelism uint8
	salt        []byte
	digest      []byte
}

func validPassword(password string) bool {
	return validUTF8Runes(password, passwordMinRunes, passwordMaxRunes)
}

func validNonEmptyPassword(password string) bool {
	return validUTF8Runes(password, verifyMinRunes, passwordMaxRunes)
}

func validVerifyPassword(password string) bool {
	return validUTF8Runes(password, verifyMinRunes, passwordMaxRunes)
}

func validUTF8Runes(value string, min, max int) bool {
	if !utf8.ValidString(value) {
		return false
	}
	length := utf8.RuneCountInString(value)
	return length >= min && length <= max
}

func passwordEqual(left, right string) bool {
	leftDigest := sha256.Sum256([]byte(left))
	rightDigest := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftDigest[:], rightDigest[:]) == 1
}

func encodeArgonHash(salt, digest []byte) string {
	encoding := base64.RawStdEncoding
	return "$argon2id$v=19$m=65536,t=3,p=4$" + encoding.EncodeToString(salt) + "$" + encoding.EncodeToString(digest)
}

func parseArgonHash(encoded string) (parsedArgonHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希编码不合法")
	}

	var parsed parsedArgonHash
	var seenMemory, seenTime, seenParallel bool
	for _, parameter := range strings.Split(parts[3], ",") {
		name, value, ok := strings.Cut(parameter, "=")
		if !ok || value == "" {
			return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不合法")
		}
		parsedValue, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsedValue == 0 {
			return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不合法")
		}
		switch name {
		case "m":
			if seenMemory {
				return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数重复")
			}
			parsed.memory, seenMemory = uint32(parsedValue), true
		case "t":
			if seenTime {
				return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数重复")
			}
			parsed.time, seenTime = uint32(parsedValue), true
		case "p":
			if seenParallel || parsedValue > uint64(^uint8(0)) {
				return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不合法")
			}
			parsed.parallelism, seenParallel = uint8(parsedValue), true
		default:
			return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希参数不支持")
		}
	}
	if !seenMemory || !seenTime || !seenParallel || parsed.memory > argonMemoryKiB || parsed.time > argonTime || parsed.parallelism > argonParallelism || parsed.memory < 8*uint32(parsed.parallelism) {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希成本不合法")
	}

	var err error
	parsed.salt, err = decodeArgonBase64(parts[4])
	if err != nil || len(parsed.salt) < argonMinSaltBytes || len(parsed.salt) > argonMaxSaltBytes {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希盐不合法")
	}
	parsed.digest, err = decodeArgonBase64(parts[5])
	if err != nil || len(parsed.digest) == 0 || len(parsed.digest) > argonMaxHashBytes {
		return parsedArgonHash{}, platform.NewError(platform.CodeIntegrityError, false, "口令哈希摘要不合法")
	}
	return parsed, nil
}

func decodeArgonBase64(value string) ([]byte, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

func credentialContext(ctx context.Context, op OperationContext) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "口令计算上下文缺失")
	}
	if err := credentialContextError(ctx.Err()); err != nil {
		return nil, nil, err
	}
	if op.Deadline == "" {
		return ctx, func() {}, nil
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return nil, nil, err
	}
	if !deadline.After(time.Now()) {
		return nil, nil, platform.NewError(platform.CodeDeadlineExceeded, true, "口令计算已超时")
	}
	workCtx, cancel := context.WithDeadline(ctx, deadline)
	return workCtx, cancel, nil
}

func acquirePasswordSlot(ctx context.Context) error {
	select {
	case passwordSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return credentialContextError(ctx.Err())
	}
}

func credentialContextError(err error) error {
	switch err {
	case nil:
		return nil
	case context.Canceled:
		return platform.NewError(platform.CodeCancelled, true, "口令计算已取消")
	case context.DeadlineExceeded:
		return platform.NewError(platform.CodeDeadlineExceeded, true, "口令计算已超时")
	default:
		return platform.NewError(platform.CodeDependencyUnavailable, true, "口令计算上下文不可用")
	}
}

// ChangePasswordInput 只包含口令与凭据CAS，主体固定取可信当前会话。
type ChangePasswordInput struct {
	OldPassword                string            `json:"oldPassword"`
	NewPassword                Password          `json:"newPassword"`
	Confirmation               Password          `json:"confirmation"`
	ExpectedCredentialRevision platform.Revision `json:"expectedCredentialRevision"`
}
type ChangePasswordResult struct {
	Account              Account       `json:"account"`
	CurrentSessionTokens SessionTokens `json:"currentSessionTokens"`
}
type sensitiveOperationReceipt struct {
	AccountID          string            `json:"accountId"`
	SessionID          string            `json:"sessionId"`
	CredentialRevision platform.Revision `json:"credentialRevision"`
}

// ChangePassword 原子更新凭据与当前设备，并撤销其他设备；回执只保存安全引用。
func ChangePassword(ctx context.Context, op OperationContext, input ChangePasswordInput) (ChangePasswordResult, error) {
	valid, err := ValidatePasswordChange(ValidatePasswordChangeInput{OldPassword: input.OldPassword, NewPassword: input.NewPassword, Confirmation: input.Confirmation})
	if err != nil {
		return ChangePasswordResult{}, err
	}
	if !valid.Valid {
		return ChangePasswordResult{}, platform.NewError(platform.CodeValidation, false, "新口令须一致且与旧口令不同")
	}
	store, ok := identityStoreFromContext(ctx)
	if !ok {
		return ChangePasswordResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "账户存储尚未装配")
	}
	digest, keyID, err := privateIdentityDigest(ctx, op, "changePassword", input)
	if err != nil {
		return ChangePasswordResult{}, err
	}
	readTx, err := store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ChangePasswordResult{}, identityDBError()
	}
	var receipt sensitiveOperationReceipt
	found, err := readIdentityReceipt(ctx, op, readTx, digest, keyID, &receipt)
	if err != nil {
		readTx.Rollback()
		return ChangePasswordResult{}, err
	}
	if found {
		readTx.Rollback()
		return ChangePasswordResult{}, sensitiveReplayError()
	}
	snapshot, _, err := readActorAccount(ctx, op, readTx, "account.self", "identity:changePassword")
	if err != nil {
		readTx.Rollback()
		return ChangePasswordResult{}, err
	}
	if snapshot.CurrentCredentialRevision != input.ExpectedCredentialRevision {
		readTx.Rollback()
		return ChangePasswordResult{}, platform.NewError(platform.CodeConflict, false, "凭据版本已变化")
	}
	var passwordHash string
	err = readTx.QueryRowContext(ctx, `SELECT password_hash FROM credentials WHERE account_id=?`, op.Actor.SubjectID).Scan(&passwordHash)
	readTx.Rollback()
	if err != nil {
		return ChangePasswordResult{}, identityDBError()
	}
	verified, err := VerifyPassword(VerifyPasswordInput{Password: input.OldPassword, EncodedHash: passwordHash})
	if err != nil {
		return ChangePasswordResult{}, err
	}
	if !verified.Valid {
		return ChangePasswordResult{}, platform.NewError(platform.CodeUnauthenticated, false, "原口令不正确")
	}
	hashed, err := HashPassword(ctx, op, HashPasswordInput{Password: input.NewPassword})
	if err != nil {
		return ChangePasswordResult{}, err
	}
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return ChangePasswordResult{}, err
	}
	store.accountMu.Lock()
	defer store.accountMu.Unlock()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return ChangePasswordResult{}, identityDBError()
	}
	defer tx.Rollback()
	found, err = readIdentityReceipt(ctx, op, tx, digest, keyID, &receipt)
	if err != nil {
		return ChangePasswordResult{}, err
	}
	if found {
		return ChangePasswordResult{}, sensitiveReplayError()
	}
	snapshot, account, err := readActorAccount(ctx, op, tx, "account.self", "identity:changePassword")
	if err != nil {
		return ChangePasswordResult{}, err
	}
	if snapshot.CurrentCredentialRevision != input.ExpectedCredentialRevision {
		return ChangePasswordResult{}, platform.NewError(platform.CodeConflict, false, "凭据版本已变化")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE credentials SET password_hash=?,credential_revision=credential_revision+1 WHERE account_id=? AND credential_revision=?`, hashed.EncodedHash, account.AccountID, input.ExpectedCredentialRevision)
	if err != nil {
		return ChangePasswordResult{}, identityDBError()
	}
	if count, err := changed.RowsAffected(); err != nil || count != 1 {
		return ChangePasswordResult{}, platform.NewError(platform.CodeConflict, false, "凭据版本已变化")
	}
	account.Revision++
	account.MustChangePassword = false
	account.UpdatedAt = platform.Now()
	changed, err = tx.ExecContext(ctx, `UPDATE accounts SET must_change_password=0,revision=?,updated_at=? WHERE account_id=? AND revision=?`, account.Revision, account.UpdatedAt, account.AccountID, account.Revision-1)
	if err != nil {
		return ChangePasswordResult{}, identityDBError()
	}
	if count, err := changed.RowsAffected(); err != nil || count != 1 {
		return ChangePasswordResult{}, platform.NewError(platform.CodeConflict, false, "账户版本已变化")
	}
	inner := WithIdentityTransaction(ctx, tx)
	if _, err := RevokeOtherSessions(inner, op, RevokeOtherSessionsInput{AccountID: account.AccountID, KeepSessionID: op.Actor.SessionID}); err != nil {
		return ChangePasswordResult{}, err
	}
	current, err := readSessionSnapshot(ctx, tx, op.Actor.SessionID)
	if err != nil {
		return ChangePasswordResult{}, err
	}
	auth, err := sessionAuthContext(ctx, tx, current)
	if err != nil {
		return ChangePasswordResult{}, err
	}
	issued, err := IssueSessionTokens(inner, op, IssueSessionTokensInput{Session: current.Session, Auth: auth})
	if err != nil {
		return ChangePasswordResult{}, err
	}
	if _, err := PublishIdentityEvent(inner, op, PublishIdentityEventInput{EventID: platform.NewIdentifier(), Event: IdentityChanged{
		AccountID: account.AccountID, SiteID: account.SiteID, PermissionRevision: account.PermissionRevision,
		CredentialRevision: input.ExpectedCredentialRevision + 1, Reason: "password", OccurredAt: account.UpdatedAt}}); err != nil {
		return ChangePasswordResult{}, err
	}
	receipt = sensitiveOperationReceipt{AccountID: account.AccountID, SessionID: op.Actor.SessionID, CredentialRevision: input.ExpectedCredentialRevision + 1}
	if err := saveIdentityReceipt(ctx, op, tx, digest, keyID, receipt); err != nil {
		return ChangePasswordResult{}, err
	}
	if err := commitAccountWrite(ctx, op, store, tx); err != nil {
		return ChangePasswordResult{}, err
	}
	return ChangePasswordResult{Account: account, CurrentSessionTokens: issued.Tokens}, nil
}

func sensitiveReplayError() error {
	return platform.NewError(platform.CodeConflict, false, "该操作已提交；敏感会话响应不可重放，请重新登录")
}
