// Package assistant 维护管理员会话和只读助手审阅结果。
package assistant

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/entry"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
	"time"
)

type OperationContext = platform.OperationContext
type AppendAssistantMessageInput = gen.AppendAssistantMessageInput
type AppendAssistantMessageResult = gen.AppendAssistantMessageResult
type ReadAssistantConversationInput struct {
	ConversationId string  `json:"conversationId"`
	Cursor         *string `json:"cursor,omitempty"`
	Limit          int     `json:"limit"`
}
type ReadAssistantConversationResult = gen.ReadAssistantConversationResult

func AppendAssistantMessage(ctx context.Context, op OperationContext, input AppendAssistantMessageInput) (AppendAssistantMessageResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return AppendAssistantMessageResult{}, err
	}
	if len([]byte(input.Body)) > 8192 || entry.ContainsPrivateMaterial(input.Body) {
		return AppendAssistantMessageResult{}, platform.NewError(platform.CodeValidation, false, "助手消息过长或含禁止保存的凭证")
	}
	var result AppendAssistantMessageResult
	err := memoryruntime.Write(ctx, op, "memory.AppendAssistantMessage", input.IdempotencyKey, input, &result, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		now := time.Now().UTC()
		id := ""
		version := 0
		if input.ConversationId == nil {
			if input.ExpectedVersion != 0 {
				return platform.NewError(platform.CodeVersionMismatch, false, "新会话版本必须为0")
			}
			id = platform.NewIdentifier()
			_, err := tx.ExecContext(ctx, `INSERT INTO assistant_conversations(site_id,conversation_id,actor_id,version,expires_at) VALUES(?,?,?,0,?)`, op.Actor.SiteID, id, op.Actor.SubjectID, now.Add(30*24*time.Hour).Format(time.RFC3339Nano))
			if err != nil {
				return memoryruntime.DBError(err)
			}
		} else {
			id = *input.ConversationId
			owner, current, err := readConversation(ctx, tx, op.Actor.SiteID, id)
			if err != nil {
				return err
			}
			if owner != op.Actor.SubjectID {
				return platform.NewError(platform.CodeForbidden, false, "只能追加自己的助手会话")
			}
			version = current
			if version != input.ExpectedVersion {
				return platform.NewError(platform.CodeVersionMismatch, false, "助手会话版本已变化")
			}
		}
		version++
		message := gen.AssistantMessage{MessageId: platform.NewIdentifier(), ConversationId: id, Ordinal: version, Role: "user", Body: input.Body, CreatedAt: now}
		_, err := tx.ExecContext(ctx, `INSERT INTO assistant_messages(site_id,message_id,conversation_id,ordinal,role,body,created_at) VALUES(?,?,?,?,?,?,?)`, op.Actor.SiteID, message.MessageId, id, message.Ordinal, message.Role, message.Body, now.Format(time.RFC3339Nano))
		if err != nil {
			return memoryruntime.DBError(err)
		}
		_, err = tx.ExecContext(ctx, `UPDATE assistant_conversations SET version=?,expires_at=? WHERE site_id=? AND conversation_id=? AND version=?`, version, now.Add(30*24*time.Hour).Format(time.RFC3339Nano), op.Actor.SiteID, id, version-1)
		if err != nil {
			return memoryruntime.DBError(err)
		}
		result = AppendAssistantMessageResult{ConversationId: id, Message: message, Version: version}
		return nil
	})
	return result, err
}
func readConversation(ctx context.Context, q memoryruntime.Queryer, site, id string) (string, int, error) {
	var owner, expires string
	var version int
	err := q.QueryRowContext(ctx, `SELECT actor_id,version,expires_at FROM assistant_conversations WHERE site_id=? AND conversation_id=?`, site, id).Scan(&owner, &version, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, platform.NewError(platform.CodeNotFound, false, "助手会话不存在")
	}
	if err != nil {
		return "", 0, memoryruntime.DBError(err)
	}
	expiry, err := platform.ParseTimestamp(expires)
	if err != nil {
		return "", 0, platform.NewError(platform.CodeIntegrityError, false, "助手会话到期时间损坏")
	}
	if !expiry.After(time.Now()) {
		return "", 0, platform.NewError(platform.CodeNotFound, false, "助手会话已过保留期")
	}
	return owner, version, nil
}

type conversationCursor struct {
	Site           string `json:"site"`
	ConversationId string `json:"conversationId"`
	After          int    `json:"after"`
}

func ReadAssistantConversation(ctx context.Context, op OperationContext, input ReadAssistantConversationInput) (ReadAssistantConversationResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ReadAssistantConversationResult{}, err
	}
	result := ReadAssistantConversationResult{Messages: []gen.AssistantMessage{}}
	err := memoryruntime.ReadSnapshot(ctx, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		_, version, err := readConversation(ctx, tx, op.Actor.SiteID, input.ConversationId)
		if err != nil {
			return err
		}
		result.Version = version
		cursor := conversationCursor{Site: op.Actor.SiteID, ConversationId: input.ConversationId}
		if input.Cursor != nil {
			raw, err := base64.RawURLEncoding.DecodeString(*input.Cursor)
			if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Site != op.Actor.SiteID || cursor.ConversationId != input.ConversationId || cursor.After < 0 {
				return platform.NewError(platform.CodeValidation, false, "助手消息游标不匹配")
			}
		}
		rows, err := tx.QueryContext(ctx, `SELECT message_id,ordinal,role,body,run_id,created_at FROM assistant_messages WHERE site_id=? AND conversation_id=? AND ordinal>? ORDER BY ordinal LIMIT ?`, op.Actor.SiteID, input.ConversationId, cursor.After, input.Limit+1)
		if err != nil {
			return memoryruntime.DBError(err)
		}
		defer rows.Close()
		for rows.Next() {
			m := gen.AssistantMessage{ConversationId: input.ConversationId}
			var run sql.NullString
			var at string
			if err = rows.Scan(&m.MessageId, &m.Ordinal, &m.Role, &m.Body, &run, &at); err != nil {
				return memoryruntime.DBError(err)
			}
			m.CreatedAt, err = platform.ParseTimestamp(at)
			if err != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "助手消息时间损坏")
			}
			if run.Valid {
				m.RunId = &run.String
			}
			result.Messages = append(result.Messages, m)
		}
		if err = rows.Err(); err != nil {
			return memoryruntime.DBError(err)
		}
		if len(result.Messages) > input.Limit {
			result.Messages = result.Messages[:input.Limit]
			cursor.After = result.Messages[len(result.Messages)-1].Ordinal
			raw, _ := json.Marshal(cursor)
			next := base64.RawURLEncoding.EncodeToString(raw)
			result.NextCursor = &next
		}
		return nil
	})
	return result, err
}

type DeleteAssistantConversationInput struct {
	ConversationId string `json:"conversationId"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type DeleteAssistantConversationResult = gen.DeleteAssistantConversationResult

func DeleteAssistantConversation(ctx context.Context, op OperationContext, input DeleteAssistantConversationInput) (DeleteAssistantConversationResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return DeleteAssistantConversationResult{}, err
	}
	var result DeleteAssistantConversationResult
	err := memoryruntime.Write(ctx, op, "memory.DeleteAssistantConversation", input.IdempotencyKey, input, &result, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		owner, _, err := readConversation(ctx, tx, op.Actor.SiteID, input.ConversationId)
		if err != nil {
			return err
		}
		if owner != op.Actor.SubjectID {
			return platform.NewError(platform.CodeForbidden, false, "只能删除自己的助手会话")
		}
		rows, err := tx.QueryContext(ctx, `SELECT DISTINCT run_id FROM assistant_messages WHERE site_id=? AND conversation_id=? AND run_id IS NOT NULL`, op.Actor.SiteID, input.ConversationId)
		if err != nil {
			return memoryruntime.DBError(err)
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return memoryruntime.DBError(err)
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return memoryruntime.DBError(err)
		}
		for _, id := range ids {
			if _, err = CancelRunTransaction(ctx, tx, op, id); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM assistant_messages WHERE site_id=? AND conversation_id=?`, op.Actor.SiteID, input.ConversationId); err != nil {
			return memoryruntime.DBError(err)
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM assistant_conversations WHERE site_id=? AND conversation_id=?`, op.Actor.SiteID, input.ConversationId)
		result.Deleted = err == nil
		return memoryruntime.DBError(err)
	})
	if err != nil {
		return result, err
	}
	if err = memoryruntime.DispatchAssistantCancellations(ctx, 100); err != nil {
		return result, err
	}
	return result, nil
}
