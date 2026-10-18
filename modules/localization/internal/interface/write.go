package interfacecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
)

type PutInterfaceStringInput = gen.PutInterfaceStringInput
type PutInterfaceStringResult = gen.PutInterfaceStringResult

func PutInterfaceString(ctx context.Context, op OperationContext, input PutInterfaceStringInput) (PutInterfaceStringResult, error) {
	if err := language.RequireAdministrator(op); err != nil {
		return PutInterfaceStringResult{}, err
	}
	var result PutInterfaceStringResult
	err := language.Mutate(ctx, op, "localization.PutInterfaceString", input.IdempotencyKey, input, &result, func(tx *sql.Tx) error {
		target, err := EnabledLanguage(ctx, tx, input.Language)
		if err != nil {
			return err
		}
		if _, err = ValidateInSnapshot(ctx, tx, []gen.InterfaceString{input.String}); err != nil {
			return err
		}
		var current int
		err = tx.QueryRowContext(ctx, `SELECT version FROM interface_catalog WHERE language=? AND key=? AND data_epoch=?`, target, input.String.Key, op.DataEpoch).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return language.StorageError(err)
		}
		if current != input.ExpectedVersion {
			return platform.NewError(platform.CodeVersionMismatch, false, "界面字符串版本已变化")
		}
		value := input.String
		value.Origin = "human"
		value.Protected = true
		value.Version = current + 1
		_, err = tx.ExecContext(ctx, `UPDATE domain_versions SET catalog_generation=catalog_generation+1 WHERE singleton=1`)
		if err != nil {
			return language.StorageError(err)
		}
		var generation int
		if err = tx.QueryRowContext(ctx, `SELECT catalog_generation FROM domain_versions WHERE singleton=1`).Scan(&generation); err != nil {
			return language.StorageError(err)
		}
		placeholders, _ := json.Marshal(value.Placeholders)
		saved, err := tx.ExecContext(ctx, `INSERT INTO interface_catalog(language,key,source_hash,value_text,placeholders,generation,origin,protected,version,data_epoch,updated_at) VALUES(?,?,?,?,?,?,'human',1,?,?,?) ON CONFLICT(language,key) DO UPDATE SET source_hash=excluded.source_hash,value_text=excluded.value_text,placeholders=excluded.placeholders,generation=excluded.generation,origin='human',protected=1,version=excluded.version,updated_at=excluded.updated_at WHERE interface_catalog.data_epoch=excluded.data_epoch`, target, value.Key, value.SourceHash, value.Value, string(placeholders), generation, value.Version, op.DataEpoch, platform.Now())
		if err != nil {
			return language.StorageError(err)
		}
		changed, _ := saved.RowsAffected()
		if changed != 1 {
			return platform.NewError(platform.CodeVersionMismatch, false, "界面字符串属于不同数据代次")
		}
		merged, err := ReadMerged(ctx, tx, op, target)
		if err != nil {
			return err
		}
		raw, _ := platform.CanonicalJSON(merged)
		result.String = value
		return language.EmitReference(tx, op, "interface-catalog.changed", "interface-catalog", generation, map[string]any{"language": target, "generation": generation, "catalogHash": platform.HashBytes(raw), "dataEpoch": op.DataEpoch})
	})
	return result, err
}
