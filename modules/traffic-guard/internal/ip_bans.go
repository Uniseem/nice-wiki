package trafficguard

import (
	"context"
	"net/netip"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
	"time"
)

// bindAdmissionIP 只接受portal签名的连接地址；不把IPv6前缀作为封禁对象。
func bindAdmissionIP(ctx context.Context, op OperationContext, subject, raw string) (string, error) {
	peer, err := netip.ParseAddr(raw)
	if err != nil || peer.Zone() != "" {
		return "", platform.NewError(platform.CodeValidation, false, "准入连接地址不合法")
	}
	ip := peer.Unmap().String()
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return "", err
	}
	defer t.close()
	if _, err = t.tx.ExecContext(t.ctx, `UPDATE guard_subjects SET ip=? WHERE site_id=? AND data_epoch=? AND subject_key=?`, ip, t.site, op.DataEpoch, subject); err != nil {
		return "", loginDBError()
	}
	var banned int
	if err = t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_bans WHERE site_id=? AND data_epoch=? AND ip=? AND state='active' AND expires_at_ns>?`, t.site, op.DataEpoch, ip, t.now.UnixNano()).Scan(&banned); err != nil {
		return "", loginDBError()
	}
	if err = t.commit(); err != nil {
		return "", err
	}
	if banned > 0 {
		return "", platform.NewError(platform.CodeForbidden, false, "该IP的AI功能暂被封禁")
	}
	return ip, nil
}

// storeAdmissionIP 固定本次许可的连接IP，之后上报与撤销均按这份记录，不跟随会话的后续连接变化。
func storeAdmissionIP(ctx context.Context, op OperationContext, admissionID, ip string) error {
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return err
	}
	defer t.close()
	result, err := t.tx.ExecContext(t.ctx, `UPDATE guard_permits SET ip=? WHERE admission_id=? AND site_id=? AND data_epoch=? AND request_id=? AND (ip='' OR ip=?)`, ip, admissionID, t.site, op.DataEpoch, op.RootOperationID, ip)
	if err != nil {
		return loginDBError()
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return platform.NewError(platform.CodeConflict, false, "本次准入已绑定其他连接IP")
	}
	var banned int
	if err = t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_bans WHERE site_id=? AND data_epoch=? AND ip=? AND state='active' AND expires_at_ns>?`, t.site, op.DataEpoch, ip, t.now.UnixNano()).Scan(&banned); err != nil {
		return loginDBError()
	}
	if banned > 0 {
		if err = revokeIPPermits(t, ip); err != nil {
			return err
		}
	}
	if err = t.commit(); err != nil {
		return err
	}
	if banned > 0 {
		return platform.NewError(platform.CodeForbidden, false, "该IP的AI功能暂被封禁")
	}
	return nil
}

func revokeIPPermits(t *guardTransaction, ip string) error {
	_, err := t.tx.ExecContext(t.ctx, `UPDATE guard_permits SET state='revoked',revision=revision+1 WHERE site_id=? AND data_epoch=? AND state='active' AND owner_caller='qa' AND ip=?`, t.site, t.op.DataEpoch, ip)
	if err != nil {
		return loginDBError()
	}
	return nil
}

func createIPBan(t *guardTransaction, ip string, ttl int64, reason string) (CreateBanResult, error) {
	result := CreateBanResult{BanID: platform.NewIdentifier(), Revision: 1}
	network, err := serviceauth.PrivateOperationDigest(t.runtime.BudgetKey, "guard.ip-ban.v1", []byte(t.site+":"+ip))
	if err != nil {
		return result, err
	}
	_, err = t.tx.ExecContext(t.ctx, `INSERT INTO guard_bans(ban_id,site_id,data_epoch,network_key,scope,reason_code,starts_at_ns,expires_at_ns,state,revision,ip) VALUES(?,?,?,?,'ai',?,?,?,'active',1,?)`, result.BanID, t.site, t.op.DataEpoch, network, reason, t.now.UnixNano(), t.now.Add(time.Duration(ttl)*time.Second).UnixNano(), ip)
	if err != nil {
		return result, loginDBError()
	}
	if err = revokeIPPermits(t, ip); err != nil {
		return result, err
	}
	if err = enqueueGuardEvent(t, "traffic-guard.ban-changed.v1", result.BanID, 1, map[string]any{"banId": result.BanID, "banRevision": int64(1), "state": "active"}); err != nil {
		return result, err
	}
	return result, nil
}
