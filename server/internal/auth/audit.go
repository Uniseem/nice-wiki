package auth

import (
	"context"
	"nice-wiki/internal/store/sqlc"
	"time"
)

func (s *Service) Audit(ctx context.Context, account Account, action, target string) error {
	return s.queries.AddAudit(ctx, sqlc.AddAuditParams{AccountID: account.ID, AccountName: account.Username, Action: action, TargetID: target, CreatedAt: time.Now().Unix()})
}
func (s *Service) ListAudit(ctx context.Context, limit, offset int64) ([]sqlc.AuditLog, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.queries.ListAudit(ctx, sqlc.ListAuditParams{Limit: limit, Offset: offset})
}
