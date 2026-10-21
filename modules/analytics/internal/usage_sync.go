package analytics

import (
	"context"
	"time"
)

// SyncUsage 从模型网关读取账项（自本域已投影水位之后）并投影；整段收齐后才标记“费用统计完整”。
// 任一页失败都不投影、不标记，下一轮从同一水位重来。
func SyncUsage(ctx context.Context, op OperationContext, rt *Runtime) error {
	if rt.UsageGateway == nil {
		return nil
	}
	ctx = WithUsageGateway(ctx, rt.UsageGateway)
	var after int64
	if err := rt.DB.QueryRowContext(ctx, `SELECT coalesce((SELECT revision FROM aggregation_watermarks WHERE source='usage'),0)`).Scan(&after); err != nil {
		return storageError(err)
	}
	snapshot, err := FetchUsagePage(ctx, op, UsageGatewayFetchInput{AfterRevision: after, DataEpoch: rt.Epoch})
	if err != nil {
		return err
	}
	input := UsageProjectionInput{DataEpoch: rt.Epoch}
	for _, page := range snapshot.Pages {
		input.Changes = append(input.Changes, page.Changes...)
		input.Observations = append(input.Observations, page.Observations...)
	}
	if len(input.Changes) > 0 || len(input.Observations) > 0 {
		if _, err = ProjectUsage(ctx, op, input); err != nil {
			return err
		}
	}
	if !snapshot.CompleteFeeCoverage {
		return nil
	}
	now := time.Now().UTC().Format(analyticsTimeLayout)
	for _, source := range []string{"usage-v2-complete", "usage-reconciled"} {
		if _, err = rt.DB.ExecContext(ctx, `INSERT INTO aggregation_watermarks(source,revision,updated_at) VALUES(?,?,?) ON CONFLICT(source) DO UPDATE SET revision=excluded.revision,updated_at=excluded.updated_at`, source, snapshot.ThroughRevision, now); err != nil {
			return storageError(err)
		}
	}
	return nil
}
