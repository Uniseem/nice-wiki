package interfacecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	"sort"
)

type ReadInterfaceCatalogInput struct {
	Language        string `json:"language"`
	KnownGeneration *int   `json:"knownGeneration,omitempty"`
}
type ReadInterfaceCatalogResult = gen.ReadInterfaceCatalogResult

func EnabledLanguage(ctx context.Context, tx *sql.Tx, tag string) (string, error) {
	wanted, err := language.NormalizeLanguageTag(ctx, OperationContext{}, language.NormalizeLanguageTagInput{Tag: tag})
	if err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `SELECT language_tag,enabled FROM languages ORDER BY language_tag`)
	if err != nil {
		return "", language.StorageError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var stored string
		var enabled bool
		if err = rows.Scan(&stored, &enabled); err != nil {
			return "", language.StorageError(err)
		}
		normalized, err := language.NormalizeLanguageTag(ctx, OperationContext{}, language.NormalizeLanguageTagInput{Tag: stored})
		if err != nil {
			return "", platform.NewError(platform.CodeIntegrityError, false, "语言配置不是有效BCP47")
		}
		if normalized.Tag == wanted.Tag {
			if !enabled {
				return "", platform.NewError(platform.CodeNotFound, false, "目标语言未启用")
			}
			return wanted.Tag, nil
		}
	}
	if err = rows.Err(); err != nil {
		return "", language.StorageError(err)
	}
	return "", platform.NewError(platform.CodeNotFound, false, "目标语言未配置")
}
func ReadMerged(ctx context.Context, tx *sql.Tx, op OperationContext, target string) ([]gen.InterfaceString, error) {
	source, err := SourceCatalog(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT key,source_hash,value_text,placeholders,origin,protected,version FROM interface_catalog WHERE language=? AND data_epoch=? ORDER BY key`, target, op.DataEpoch)
	if err != nil {
		return nil, language.StorageError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var value gen.InterfaceString
		var raw []byte
		if err = rows.Scan(&value.Key, &value.SourceHash, &value.Value, &raw, &value.Origin, &value.Protected, &value.Version); err != nil {
			return nil, language.StorageError(err)
		}
		if json.Unmarshal(raw, &value.Placeholders) != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "界面字典插值索引损坏")
		}
		original, known := source[value.Key]
		if !known {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "持久字典包含未知源key")
		}
		if value.SourceHash != original.SourceHash {
			continue
		}
		if err = CheckString(value, original); err != nil {
			return nil, err
		}
		source[value.Key] = value
	}
	if err = rows.Err(); err != nil {
		return nil, language.StorageError(err)
	}
	list := make([]gen.InterfaceString, 0, len(source))
	for _, s := range source {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Key < list[j].Key })
	return list, nil
}
func ReadInterfaceCatalog(ctx context.Context, op OperationContext, input ReadInterfaceCatalogInput) (ReadInterfaceCatalogResult, error) {
	result := ReadInterfaceCatalogResult{Strings: []gen.InterfaceString{}}
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		tag, err := EnabledLanguage(ctx, tx, input.Language)
		if err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, `SELECT catalog_generation FROM domain_versions WHERE singleton=1`).Scan(&result.Generation); err != nil {
			return language.StorageError(err)
		}
		// 未安装源字典不能凭版本号伪造304。
		if _, err = SourceCatalog(ctx, tx); err != nil {
			return err
		}
		if input.KnownGeneration != nil && *input.KnownGeneration == result.Generation {
			result.NotModified = true
			return nil
		}
		result.Strings, err = ReadMerged(ctx, tx, op, tag)
		return err
	})
	return result, err
}
