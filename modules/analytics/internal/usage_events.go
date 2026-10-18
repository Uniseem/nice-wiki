package analytics

import (
	"encoding/json"

	"nice-wiki/libs/platform"
)

// usageEventReference只提取定位权威费用事实所需的原引用，不读取事件金额。
type usageEventReference struct {
	InvocationID    string `json:"invocationId"`
	AttemptID       string `json:"attemptId"`
	RootOperationID string `json:"rootOperationId"`
	CostClass       string `json:"costClass"`
	EntryID         string `json:"entryId"`
	ProofKind       string `json:"proofKind"`
	DataEpoch       string `json:"dataEpoch"`
	SourceDataEpoch string `json:"sourceDataEpoch"`
}

// parseUsageEventReference接在平台schema校验之后，仅核本入口的生产者、版本及原引用绑定。
func parseUsageEventReference(event platform.EventEnvelope, epoch string) (usageEventReference, error) {
	if event.Producer != "model-gateway" || event.EventVersion != 1 || (event.EventType != "invocation.settled" && event.EventType != "invocation.reconciled") {
		return usageEventReference{}, platform.NewError(platform.CodeForbidden, false, "费用事件生产者、类型或版本不匹配")
	}
	if event.DataEpoch != epoch {
		return usageEventReference{}, platform.NewError(platform.CodeVersionMismatch, false, "费用事件代次不匹配")
	}
	var ref usageEventReference
	if err := json.Unmarshal(event.Payload, &ref); err != nil {
		return usageEventReference{}, platform.NewError(platform.CodeIntegrityError, false, "费用事件引用不可读")
	}
	if ref.DataEpoch != event.DataEpoch || ref.InvocationID != event.SubjectID {
		return usageEventReference{}, platform.NewError(platform.CodeIntegrityError, false, "费用事件载荷与信封引用不一致")
	}
	if event.EventType == "invocation.settled" && ref.EntryID == "" || event.EventType == "invocation.reconciled" && (ref.ProofKind != "charged" && ref.ProofKind != "not_charged" || (ref.ProofKind == "charged") != (ref.EntryID != "")) {
		return usageEventReference{}, platform.NewError(platform.CodeIntegrityError, false, "费用事件缺对应证明种类的原账项引用")
	}
	if ref.SourceDataEpoch == "" {
		ref.SourceDataEpoch = ref.DataEpoch
	}
	return ref, nil
}

// validateUsageEventReference只绑定原引用，事件金额和event.Revision均不成为账项事实。
// 高于事件的权威事实仍然有效，例如迟到settled对应的账项已被supersede或void。
func validateUsageEventReference(ref usageEventReference, siteID string, changes []UsageLedgerChange, observations []UsageObservation) error {
	entryFound := false
	for _, change := range changes {
		if ref.EntryID == "" || change.LedgerEntryID != ref.EntryID {
			continue
		}
		if change.InvocationID != ref.InvocationID || change.AttemptID != ref.AttemptID || change.RootOperationID != ref.RootOperationID || change.CostClass != ref.CostClass || change.SiteID != siteID || change.SourceDataEpoch != ref.SourceDataEpoch {
			return platform.NewError(platform.CodeIntegrityError, false, "权威账项与费用事件原引用不一致")
		}
		entryFound = true
	}
	if ref.EntryID != "" {
		if !entryFound {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "完整权威费用页尚无事件所引账项，不确认事件")
		}
		return nil
	}
	// not_charged不生成账项，只能由权威页中的原attempt事实确认原引用存在。
	attemptFound := false
	for _, observation := range observations {
		if observation.Kind != "attempt" || observation.Attempt == nil || observation.Attempt.AttemptID != ref.AttemptID {
			continue
		}
		origin := observation.Origin
		if origin.InvocationID != ref.InvocationID || origin.RootOperationID != ref.RootOperationID || origin.CostClass != ref.CostClass || origin.SiteID != siteID || origin.SourceDataEpoch != ref.SourceDataEpoch {
			return platform.NewError(platform.CodeIntegrityError, false, "权威尝试与费用事件原引用不一致")
		}
		attemptFound = true
	}
	if attemptFound {
		return nil
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, "完整权威费用页尚无事件所引尝试，不确认事件")
}
