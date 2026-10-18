package entry

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	memorycontext "nice-wiki/modules/memory/internal/context"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
	"time"
)

type CreateEntryInput = gen.CreateEntryInput
type CreateEntryResult = gen.CreateEntryResult
type UpdateEntryInput = gen.UpdateEntryInput
type UpdateEntryResult = gen.UpdateEntryResult
type ArchiveEntryInput = gen.ArchiveEntryInput
type ArchiveEntryResult = gen.ArchiveEntryResult
type RestoreEntryInput = gen.RestoreEntryInput
type RestoreEntryResult = gen.RestoreEntryResult

func CreateEntry(ctx context.Context, op OperationContext, input CreateEntryInput) (CreateEntryResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return CreateEntryResult{}, err
	}
	var result CreateEntryResult
	err := memoryruntime.Write(ctx, op, "memory.CreateEntry", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		var exists int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM entry_current WHERE site_id=? AND entry_id=?`, op.Actor.SiteID, input.Entry.EntryId).Scan(&exists)
		if err == nil {
			return platform.NewError(platform.CodeConflict, false, "记忆条目已经存在")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return memoryruntime.DBError(err)
		}
		e := input.Entry
		if e.Version != 0 {
			return platform.NewError(platform.CodeValidation, false, "新记忆条目必须为版本0")
		}
		e.State = "active"
		e.Source = "manual"
		e.AutoUpdateEnabled = false
		result.Entry, err = PersistVersion(ctx, s, tx, op, e, nil)
		return err
	})
	return result, err
}
func UpdateEntry(ctx context.Context, op OperationContext, input UpdateEntryInput) (UpdateEntryResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return UpdateEntryResult{}, err
	}
	var result UpdateEntryResult
	err := memoryruntime.Write(ctx, op, "memory.UpdateEntry", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		current, err := s.ReadEntryObject(ctx, tx, op.Actor.SiteID, input.Entry.EntryId, nil)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return platform.NewError(platform.CodeVersionMismatch, false, "记忆版本已变化")
		}
		if current.State != "active" {
			return platform.NewError(platform.CodeConflict, false, "归档记忆须先恢复")
		}
		e := input.Entry
		e.Version = current.Version + 1
		e.State = current.State
		e.Source = "manual"
		e.AutoUpdateEnabled = false
		result.Entry, err = PersistVersion(ctx, s, tx, op, e, &current.Version)
		return err
	})
	return result, err
}
func PersistVersion(ctx context.Context, s *memoryruntime.Store, tx *sql.Tx, op OperationContext, e gen.MemoryEntry, previous *int) (gen.MemoryEntry, error) {
	normalized, err := ValidateEntry(e)
	if err != nil {
		return gen.MemoryEntry{}, err
	}
	normalized.UpdatedAt = time.Now().UTC()
	projection, err := memorycontext.ValidateProjectedBudgets(ctx, s, tx, op.Actor.SiteID, &normalized, nil, nil)
	if err != nil {
		return gen.MemoryEntry{}, err
	}
	saved, err := s.SaveEntryObject(ctx, tx, op.Actor.SiteID, normalized)
	if err != nil {
		return gen.MemoryEntry{}, err
	}
	if err = memoryruntime.InsertEntryVersion(ctx, tx, op, saved, previous); err != nil {
		return gen.MemoryEntry{}, err
	}
	if err = memoryruntime.Emit(tx, op, "memory.entry.changed", saved.EntryId, saved.Version, map[string]any{"entryId": saved.EntryId, "version": saved.Version, "bodyHash": saved.BodyHash, "affectedTaskIds": projection.AffectedTaskIds}); err != nil {
		return gen.MemoryEntry{}, err
	}
	if err = memorycontext.EmitContextChanged(ctx, tx, op, projection.AffectedTaskIds); err != nil {
		return gen.MemoryEntry{}, err
	}
	return saved, nil
}
func ArchiveEntry(ctx context.Context, op OperationContext, input ArchiveEntryInput) (ArchiveEntryResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ArchiveEntryResult{}, err
	}
	var result ArchiveEntryResult
	err := memoryruntime.Write(ctx, op, "memory.ArchiveEntry", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		current, err := s.ReadEntryObject(ctx, tx, op.Actor.SiteID, input.EntryId, nil)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return platform.NewError(platform.CodeVersionMismatch, false, "记忆版本已变化")
		}
		if current.State != "active" {
			return platform.NewError(platform.CodeConflict, false, "记忆已归档")
		}
		previous := current.Version
		current.Version++
		current.State = "archived"
		current.AutoUpdateEnabled = false
		result.Entry, err = PersistVersion(ctx, s, tx, op, current, &previous)
		return err
	})
	return result, err
}
func RestoreEntry(ctx context.Context, op OperationContext, input RestoreEntryInput) (RestoreEntryResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return RestoreEntryResult{}, err
	}
	var result RestoreEntryResult
	err := memoryruntime.Write(ctx, op, "memory.RestoreEntry", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		current, err := s.ReadEntryObject(ctx, tx, op.Actor.SiteID, input.EntryId, nil)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return platform.NewError(platform.CodeVersionMismatch, false, "记忆版本已变化")
		}
		if current.State != "archived" {
			return platform.NewError(platform.CodeConflict, false, "记忆未归档")
		}
		previous := current.Version
		current.Version++
		current.State = "active"
		current.AutoUpdateEnabled = false
		result.Entry, err = PersistVersion(ctx, s, tx, op, current, &previous)
		return err
	})
	return result, err
}
