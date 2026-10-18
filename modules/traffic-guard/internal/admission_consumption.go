package trafficguard

import (
	"context"
	"nice-wiki/libs/platform"
)

type ConsumeAdmissionInput struct {
	AdmissionID   string
	RequestID     string
	CallerService string
	DataEpoch     string
}
type ConsumeAdmissionResult struct {
	Consumed bool                     `json:"consumed"`
	Permit   platform.AdmissionPermit `json:"permit"`
}

// ConsumeAdmission 不接收token；只在持久原根消费状态可提交后生成签名凭证。
func ConsumeAdmission(ctx context.Context, op OperationContext, in ConsumeAdmissionInput) (ConsumeAdmissionResult, error) {
	if op.CallerService != "qa" || in.CallerService != op.CallerService || in.RequestID != op.RootOperationID {
		return ConsumeAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "只有原QA问题根可消费许可")
	}
	t, err := beginGuardTransaction(ctx, op, true)
	if err != nil {
		return ConsumeAdmissionResult{}, err
	}
	defer t.close()
	row, err := readPermitRecord(t, in.AdmissionID)
	if err != nil {
		return ConsumeAdmissionResult{}, err
	}
	if row.owner != "qa" || row.requestID != in.RequestID {
		return ConsumeAdmissionResult{}, platform.NewError(platform.CodeConflict, false, "许可已绑定其他拥有者或请求")
	}
	if row.state != "active" || row.expires <= t.now.UnixNano() {
		return ConsumeAdmissionResult{}, platform.NewError(platform.CodeStaleResult, false, "已终止许可不能消费或复活")
	}
	banned, _, err := CheckBan(t, row.networks)
	if err != nil {
		return ConsumeAdmissionResult{}, err
	}
	if banned {
		return ConsumeAdmissionResult{}, platform.NewError(platform.CodeForbidden, false, "该网络AI功能暂被限制")
	}
	if row.consumed != "" && row.consumed != in.RequestID {
		return ConsumeAdmissionResult{}, platform.NewError(platform.CodeConflict, false, "许可已经由其他根消费")
	}
	if row.consumed == "" {
		_, err = t.tx.ExecContext(t.ctx, `UPDATE guard_permits SET consumed_request_id=?,revision=revision+1
   WHERE admission_id=? AND site_id=? AND data_epoch=? AND revision=? AND state='active' AND consumed_request_id IS NULL`, in.RequestID, row.admissionID, t.site, op.DataEpoch, row.revision)
		if err != nil {
			return ConsumeAdmissionResult{}, loginDBError()
		}
	}
	permit := permitMetadata(row, op.DataEpoch)
	if row.action == "qa.ask" || row.action == "qa.followup" {
		signed, err := SignPermit(t.ctx, op, SignPermitInput{AdmissionID: row.admissionID})
		if err != nil {
			return ConsumeAdmissionResult{}, err
		}
		permit = signed.Permit
	}
	if err = t.commit(); err != nil {
		return ConsumeAdmissionResult{}, err
	}
	return ConsumeAdmissionResult{Consumed: true, Permit: permit}, nil
}
