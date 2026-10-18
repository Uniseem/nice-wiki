package trafficguard

import (
	"context"
	"nice-wiki/libs/platform"
)

type ComputeRiskInput struct {
	Denials10m           int64
	ChallengePassed      bool
	ActiveBan            bool
	ExactRepeatCount     int64
	SimilarityScore      *int64
	RegularIntervalCount int64
	ModelVerdict         string
}
type ComputeRiskResult struct {
	Decision   string `json:"decision"`
	Difficulty int    `json:"difficulty"`
}

// ComputeRisk 不另加模型或未确认数值阈值，程序信号供QA原有前置共同判定。
func ComputeRisk(ctx context.Context, op OperationContext, in ComputeRiskInput) (ComputeRiskResult, error) {
	if in.ActiveBan || in.ModelVerdict == "abuse" {
		return ComputeRiskResult{"deny", 22}, nil
	}
	switch in.ModelVerdict {
	case "suspicious":
		if in.ChallengePassed {
			return ComputeRiskResult{"allow", 22}, nil
		}
		return ComputeRiskResult{"challenge", 22}, nil
	case "normal":
		if in.ChallengePassed {
			return ComputeRiskResult{"allow", 18}, nil
		}
		return ComputeRiskResult{"challenge", 18}, nil
	default:
		return ComputeRiskResult{}, platform.NewError(platform.CodeValidation, false, "INVALID_SIGNALS")
	}
}
