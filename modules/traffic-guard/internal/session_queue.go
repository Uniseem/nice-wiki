package trafficguard

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
)

type QueueSessionAdmissionInput struct {
	SessionKey string `json:"sessionKey"`
	RequestID  string `json:"requestId"`
	Mode       string `json:"mode,omitempty"`
	DataEpoch  string `json:"-"`
}
type QueueSessionAdmissionResult struct {
	Queued   bool  `json:"queued"`
	Position int64 `json:"position"`
}

// QueueSessionAdmission 只保留FIFO引用；正文仍在QA，排队时不消耗频率或费用。
func QueueSessionAdmission(ctx context.Context, op OperationContext, in QueueSessionAdmissionInput) (QueueSessionAdmissionResult, error) {
	if op.CallerService != "qa" {
		return QueueSessionAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "仅QA可维护问题队列引用")
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return QueueSessionAdmissionResult{}, err
	}
	defer t.close()
	var known int
	if t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_subjects WHERE site_id=? AND data_epoch=? AND session_key=?`, t.site, op.DataEpoch, in.SessionKey).Scan(&known) != nil {
		return QueueSessionAdmissionResult{}, loginDBError()
	}
	if known == 0 {
		return QueueSessionAdmissionResult{}, platform.NewError(platform.CodeNotFound, false, "匿名会话不存在")
	}
	if in.Mode == "cancel" {
		_, err = t.tx.ExecContext(t.ctx, `DELETE FROM guard_waiters WHERE site_id=? AND data_epoch=? AND session_key=? AND request_id=?`, t.site, op.DataEpoch, in.SessionKey, in.RequestID)
		if err != nil {
			return QueueSessionAdmissionResult{}, loginDBError()
		}
		if err = t.commit(); err != nil {
			return QueueSessionAdmissionResult{}, err
		}
		return QueueSessionAdmissionResult{Queued: false, Position: 0}, nil
	}
	var active, waiting int64
	if t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_permits WHERE site_id=? AND data_epoch=? AND session_key=? AND owner_caller='qa' AND state='active' AND expires_at_ns>?`, t.site, op.DataEpoch, in.SessionKey, t.now.UnixNano()).Scan(&active) != nil {
		return QueueSessionAdmissionResult{}, loginDBError()
	}
	if t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_waiters WHERE site_id=? AND data_epoch=? AND session_key=?`, t.site, op.DataEpoch, in.SessionKey).Scan(&waiting) != nil {
		return QueueSessionAdmissionResult{}, loginDBError()
	}
	var sequence int64
	err = t.tx.QueryRowContext(t.ctx, `SELECT sequence FROM guard_waiters WHERE site_id=? AND data_epoch=? AND session_key=? AND request_id=?`, t.site, op.DataEpoch, in.SessionKey, in.RequestID).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		if active == 0 && waiting == 0 {
			return QueueSessionAdmissionResult{Queued: false, Position: 0}, nil
		}
		if waiting >= 10 {
			return QueueSessionAdmissionResult{}, platform.NewError(platform.CodeRateLimited, true, "该会话等待队列已满")
		}
		inserted, err := t.tx.ExecContext(t.ctx, `INSERT INTO guard_waiters(site_id,data_epoch,session_key,request_id,created_at_ns) VALUES(?,?,?,?,?)`, t.site, op.DataEpoch, in.SessionKey, in.RequestID, t.now.UnixNano())
		if err != nil {
			return QueueSessionAdmissionResult{}, loginDBError()
		}
		sequence, err = inserted.LastInsertId()
		if err != nil {
			return QueueSessionAdmissionResult{}, loginDBError()
		}
	} else if err != nil {
		return QueueSessionAdmissionResult{}, loginDBError()
	}
	var position int64
	if t.tx.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM guard_waiters WHERE site_id=? AND data_epoch=? AND session_key=? AND sequence<=?`, t.site, op.DataEpoch, in.SessionKey, sequence).Scan(&position) != nil {
		return QueueSessionAdmissionResult{}, loginDBError()
	}
	result := QueueSessionAdmissionResult{Queued: active > 0 || position > 1, Position: position}
	if !result.Queued {
		result.Position = 0
	}
	if err = t.commit(); err != nil {
		return QueueSessionAdmissionResult{}, err
	}
	return result, nil
}
