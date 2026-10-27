package runtime

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"strconv"
)

// IsRestoredEpoch 判断记录的数据代次是否就是目标代次；恢复协议已移除，不再有跨代次投影。
func IsRestoredEpoch(_ context.Context, _ *sql.Tx, source, target string) (bool, error) {
	return source == target, nil
}

// IsRedactedMaterial 恢复协议移除后没有“备份时明确省略的密钥”，恒为否。
func IsRedactedMaterial(_ context.Context, _ *sql.Tx, _, _ string) (bool, error) {
	return false, nil
}

// CheckProviderMaterial只在新接受事务核真实当前加密材料；明确脱密要求管理员新版本配置，不按空密钥派发。
func CheckProviderMaterial(ctx context.Context, tx *sql.Tx, op OperationContext, provider gen.Provider) error {
	var epoch string
	var nonce, ciphertext int
	err := tx.QueryRowContext(ctx, `SELECT data_epoch,length(nonce),length(ciphertext) FROM credentials WHERE provider_id=? AND credential_version=?`, provider.ProviderId, provider.CredentialVersion).Scan(&epoch, &nonce, &ciphertext)
	if err == sql.ErrNoRows {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "渠道没有当前凭据material")
	}
	if err != nil {
		return ExecutionStorageError(err)
	}
	if epoch != op.DataEpoch {
		return platform.NewError(platform.CodeStaleResult, false, "当前凭据代际不同")
	}
	if nonce != 12 || ciphertext <= 16 {
		redacted, e := IsRedactedMaterial(ctx, tx, "credential", provider.ProviderId+"|"+strconv.Itoa(provider.CredentialVersion))
		if e != nil {
			return e
		}
		if redacted && nonce == 0 && ciphertext == 0 {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "备份未包含渠道密钥，需管理员配置新版本")
		}
		return platform.NewError(platform.CodeIntegrityError, false, "渠道加密材料损坏")
	}
	for i, ref := range provider.ExtraHeaderSecretRefs {
		var owner, name string
		err = tx.QueryRowContext(ctx, `SELECT provider_id,header_name,data_epoch,length(nonce),length(ciphertext) FROM header_secrets WHERE ref_id=?`, ref).Scan(&owner, &name, &epoch, &nonce, &ciphertext)
		if err != nil {
			return ExecutionStorageError(err)
		}
		projected, e := IsRestoredEpoch(ctx, tx, epoch, op.DataEpoch)
		if e != nil {
			return e
		}
		if owner != provider.ProviderId || i >= len(provider.ExtraHeaderNames) || name != provider.ExtraHeaderNames[i] || !projected {
			return platform.NewError(platform.CodeIntegrityError, false, "渠道额外头来源不属于本次配置")
		}
		if nonce != 12 || ciphertext <= 16 {
			redacted, e := IsRedactedMaterial(ctx, tx, "header", ref)
			if e != nil {
				return e
			}
			if redacted && nonce == 0 && ciphertext == 0 {
				return platform.NewError(platform.CodeDependencyUnavailable, false, "备份未包含额外头密钥，需管理员重新配置")
			}
			return platform.NewError(platform.CodeIntegrityError, false, "渠道额外头加密材料损坏")
		}
	}
	return nil
}
