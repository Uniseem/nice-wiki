package guard

import "errors"

var (
	ErrVerificationRequired = errors.New("需要重新完成人机验证")
	ErrBanned               = errors.New("AI 问答已暂时封禁")
	ErrOutOfScope           = errors.New("问题与站点内容无关")
)

type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

func Combine(frequency, repetition bool, model Risk) Risk {
	if model == RiskHigh || frequency && repetition {
		return RiskHigh
	}
	if model == RiskMedium || frequency || repetition {
		return RiskMedium
	}
	return RiskLow
}
