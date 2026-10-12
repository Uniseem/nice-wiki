package guard

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
