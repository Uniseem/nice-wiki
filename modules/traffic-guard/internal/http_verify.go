package trafficguard

import (
	"context"
	"nice-wiki/libs/platform"
	"time"
)

type VerifyAdmissionHTTPInput struct {
	AdmissionID string
	RequestID   string
	DataEpoch   string
	Token       *string
}
type VerifyAdmissionHTTPResult struct {
	AdmissionID string `json:"admissionId"`
	Valid       bool   `json:"valid"`
	ExpiresAt   string `json:"expiresAt"`
}

func VerifyAdmissionHTTP(ctx context.Context, op OperationContext, in VerifyAdmissionHTTPInput) (VerifyAdmissionHTTPResult, error) {
	if op.CallerService != "qa" || in.RequestID != op.RootOperationID {
		return VerifyAdmissionHTTPResult{}, platform.NewError(platform.CodeForbidden, false, "只允许原QA问题根验证")
	}
	if in.Token != nil {
		checked, err := ValidatePermit(ctx, op, ValidatePermitInput{Token: *in.Token, ExpectedAction: "", CallerService: op.CallerService, DataEpoch: op.DataEpoch, RequestID: in.RequestID})
		if err != nil {
			return VerifyAdmissionHTTPResult{}, err
		}
		if checked.AdmissionID != in.AdmissionID {
			return VerifyAdmissionHTTPResult{}, platform.NewError(platform.CodeForbidden, false, "签名许可与路径不一致")
		}
		return VerifyAdmissionHTTPResult{AdmissionID: checked.AdmissionID, Valid: checked.Valid, ExpiresAt: checked.ExpiresAt}, nil
	}
	t, err := beginGuardTransaction(ctx, op, false)
	if err != nil {
		return VerifyAdmissionHTTPResult{}, err
	}
	defer t.close()
	row, err := readPermitRecord(t, in.AdmissionID)
	if err != nil {
		return VerifyAdmissionHTTPResult{}, err
	}
	if row.owner != "qa" || row.requestID != in.RequestID || row.consumed != in.RequestID {
		return VerifyAdmissionHTTPResult{}, platform.NewError(platform.CodeForbidden, false, "许可不属于当前QA问题")
	}
	result := VerifyAdmissionHTTPResult{AdmissionID: row.admissionID, Valid: row.state == "active" && row.expires > t.now.UnixNano(), ExpiresAt: time.Unix(0, row.expires).UTC().Format(time.RFC3339Nano)}
	if !result.Valid {
		return result, nil
	}
	banned, _, err := CheckBan(t, row.networks)
	if err != nil {
		return VerifyAdmissionHTTPResult{}, err
	}
	result.Valid = !banned
	return result, nil
}
