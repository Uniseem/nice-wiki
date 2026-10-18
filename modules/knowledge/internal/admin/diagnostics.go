package admin

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type RedactDiagnosticsInput struct {
	BuildID       string `json:"buildId"`
	IncludeCounts bool   `json:"includeCounts"`
}
type RedactDiagnosticsResult = gen.RedactDiagnosticsResult

// RedactDiagnostics 仅投影固定安全状态、空间身份与计数，不读取源文本或触发模型。
func RedactDiagnostics(ctx context.Context, op OperationContext, input RedactDiagnosticsInput) (RedactDiagnosticsResult, error) {
	tx, err := beginAdminRead(ctx, op)
	if err != nil {
		return RedactDiagnosticsResult{}, err
	}
	defer tx.Rollback()
	record, err := indexer.ReadBuildRecord(ctx, tx, op, input.BuildID)
	if err != nil {
		return RedactDiagnosticsResult{}, err
	}
	result := RedactDiagnosticsResult{BuildId: input.BuildID, State: string(record.Build.State), SpaceId: record.Build.Space.SpaceId, Counts: make([]int, 0)}
	if input.IncludeCounts {
		var chunks, terms, vectors int
		if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM chunks WHERE build_id=?),(SELECT COUNT(*) FROM vectors WHERE build_id=?),(SELECT COUNT(*) FROM terms WHERE build_id=?)`, input.BuildID, input.BuildID, input.BuildID).Scan(&chunks, &vectors, &terms); err != nil {
			return RedactDiagnosticsResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "索引诊断计数读取失败")
		}
		result.Counts = []int{chunks, vectors, terms}
	}
	return result, nil
}
