// Package internal 提供 jobs 域内部的任务校验与状态规则。
package internal

import (
	"context"
	"mime"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

const (
	maxJobStringRunes = 2048
)

// OperationContext 是服务边界构建的可信操作上下文。
// jobs 不从请求正文或客户端字段构造该类型。
type OperationContext = platform.OperationContext

// ObjectRef 是所属服务提供的不可变对象引用。
type ObjectRef = gen.ObjectRef

// ValidateJobSpecInput 是任务入口的纯校验输入。
type ValidateJobSpecInput struct {
	Owner     string
	Type      string
	InputRef  ObjectRef
	DataEpoch platform.Identifier
}

// ValidateJobSpecResult 是任务入口校验结果。
type ValidateJobSpecResult struct {
	Valid bool
}

// registeredJobTypes 是 config/job-types.json 当前生成快照中的 owner/type
// 白名单。任务进入持久队列前必须命中精确配对，不能只校验 owner 或 type
// 的任一方；配置装配与生成流程后续会把它替换为启动时加载的登记表。
var registeredJobTypes = map[string]map[string]struct{}{
	"analytics": {
		"analytics.aggregate-daily":  {},
		"analytics.aggregate-hourly": {},
		"analytics.aggregate-usage":   {},
		"analytics.reconcile-usage":   {},
	},
	"data-protection": {
		"data-protection.backup":  {},
		"data-protection.restore": {},
	},
	"knowledge": {
		"knowledge.build": {},
		"knowledge.extract": {},
	},
	"localization": {
		"localization.cache-answer": {},
		"localization.document":     {},
		"localization.ui":           {},
	},
	"media": {
		"media.cleanup": {},
	},
	"memory": {
		"memory.assistant": {},
		"memory.overview":  {},
	},
	"public-site": {
		"public-site.render-publication": {},
	},
	"qa": {
		"qa.cache-embedding": {},
		"qa.compact":         {},
		"qa.turn":            {},
	},
}

// ValidateJobSpec 验证任务入口，不写任务状态、不读写对象内容，也不调用
// 外部服务。当前 epoch 由可信入口装入 op.DataEpoch；不接受客户端自声明的
// epoch 作为运行权威。
func ValidateJobSpec(ctx context.Context, op OperationContext, in ValidateJobSpecInput) (ValidateJobSpecResult, error) {
	if err := contextError(ctx); err != nil {
		return ValidateJobSpecResult{}, err
	}
	if err := validateJobSpecInput(op, in); err != nil {
		return ValidateJobSpecResult{}, err
	}
	if err := contextError(ctx); err != nil {
		return ValidateJobSpecResult{}, err
	}
	return ValidateJobSpecResult{Valid: true}, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "任务校验上下文缺失")
	}
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "任务校验已超过截止时间")
		}
		return platform.NewError(platform.CodeCancelled, true, "任务校验已取消")
	default:
		return nil
	}
}

func validateJobSpecInput(op OperationContext, in ValidateJobSpecInput) error {
	if !validJobString(in.Owner) || !validJobString(in.Type) {
		return invalidJob()
	}
	if op.CallerService == "" || op.CallerService != in.Owner {
		return invalidJob()
	}
	if jobs, ok := registeredJobTypes[in.Owner]; !ok || !containsJobType(jobs, in.Type) {
		return invalidJob()
	}
	if err := validateInputRef(in.Owner, in.InputRef); err != nil {
		return err
	}
	if !platform.ValidIdentifier(in.DataEpoch) || !platform.ValidIdentifier(op.DataEpoch) {
		return epochMismatch()
	}
	if in.DataEpoch != op.DataEpoch {
		return epochMismatch()
	}
	return nil
}

func validJobString(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= maxJobStringRunes
}

func containsJobType(registered map[string]struct{}, jobType string) bool {
	_, ok := registered[jobType]
	return ok
}

func validateInputRef(owner string, ref ObjectRef) error {
	if ref.Owner != owner || !platform.ValidIdentifier(ref.ObjectId) || ref.Version < 0 {
		return invalidJob()
	}
	if !platform.ValidHash(ref.Sha256) || ref.ByteSize < 0 {
		return invalidJob()
	}
	if ref.MediaType == "" {
		return invalidJob()
	}
	if _, _, err := mime.ParseMediaType(ref.MediaType); err != nil {
		return invalidJob()
	}
	return nil
}

func invalidJob() error {
	return platform.NewError(platform.CodeValidation, false, "任务规格不合法")
}

func epochMismatch() error {
	return platform.NewError(platform.CodeVersionMismatch, false, "任务数据代次与当前运行代次不一致")
}
