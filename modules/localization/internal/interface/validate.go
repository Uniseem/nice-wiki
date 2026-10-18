// Package interfacecatalog 维护固定源界面key及严格插值字典。
package interfacecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
	"regexp"
	"slices"
	"sort"
)

type OperationContext = platform.OperationContext
type ValidateInterfaceCatalogInput struct {
	Language string                `json:"language"`
	Strings  []gen.InterfaceString `json:"strings"`
}
type ValidateInterfaceCatalogResult struct {
	Valid       bool   `json:"valid"`
	CatalogHash string `json:"catalogHash"`
}

var placeholderPattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_.]*)\}`)
var htmlPattern = regexp.MustCompile(`(?i)<[/!A-Za-z][^>]*>`)

func SourceCatalog(ctx context.Context, tx *sql.Tx) (map[string]gen.InterfaceString, error) {
	rows, err := tx.QueryContext(ctx, `SELECT key,value_text,source_hash,placeholders FROM interface_sources ORDER BY key`)
	if err != nil {
		return nil, language.StorageError(err)
	}
	defer rows.Close()
	source := map[string]gen.InterfaceString{}
	for rows.Next() {
		var s gen.InterfaceString
		var raw []byte
		if err = rows.Scan(&s.Key, &s.Value, &s.SourceHash, &raw); err != nil {
			return nil, language.StorageError(err)
		}
		if json.Unmarshal(raw, &s.Placeholders) != nil || platform.HashBytes([]byte(s.Value)) != s.SourceHash {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "固定界面源目录损坏")
		}
		if err = CheckString(s, s); err != nil {
			return nil, err
		}
		s.Origin = "builtin"
		s.Protected = false
		s.Version = 0
		source[s.Key] = s
	}
	if err = rows.Err(); err != nil {
		return nil, language.StorageError(err)
	}
	if len(source) == 0 {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "固定界面源目录尚未安装")
	}
	return source, nil
}
func CheckString(value gen.InterfaceString, source gen.InterfaceString) error {
	if value.SourceHash != source.SourceHash {
		return platform.NewError(platform.CodeVersionMismatch, false, "界面字符串来源已变化")
	}
	expected := append([]string{}, source.Placeholders...)
	actual := append([]string{}, value.Placeholders...)
	sort.Strings(expected)
	sort.Strings(actual)
	captured := []string{}
	for _, m := range placeholderPattern.FindAllStringSubmatch(value.Value, -1) {
		captured = append(captured, m[1])
	}
	sort.Strings(captured)
	if !slices.Equal(expected, actual) || !slices.Equal(expected, captured) {
		return platform.NewError(platform.CodeIntegrityError, false, "界面字符串插值名称或数量与来源不一致")
	}
	if htmlPattern.MatchString(value.Value) {
		return platform.NewError(platform.CodeIntegrityError, false, "界面字符串禁止HTML")
	}
	return nil
}
func ValidateInSnapshot(ctx context.Context, tx *sql.Tx, strings []gen.InterfaceString) (string, error) {
	source, err := SourceCatalog(ctx, tx)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	for _, value := range strings {
		s, known := source[value.Key]
		if !known || seen[value.Key] {
			return "", platform.NewError(platform.CodeValidation, false, "界面字符串有未知或重复key")
		}
		seen[value.Key] = true
		if err = CheckString(value, s); err != nil {
			return "", err
		}
	}
	ordered := append([]gen.InterfaceString{}, strings...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })
	raw, err := platform.CanonicalJSON(ordered)
	if err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "界面字典编码失败")
	}
	return platform.HashBytes(raw), nil
}
func ValidateInterfaceCatalog(ctx context.Context, op OperationContext, input ValidateInterfaceCatalogInput) (ValidateInterfaceCatalogResult, error) {
	var result ValidateInterfaceCatalogResult
	err := language.ReadData(ctx, op, func(tx *sql.Tx) error {
		hash, err := ValidateInSnapshot(ctx, tx, input.Strings)
		if err != nil {
			return err
		}
		result = ValidateInterfaceCatalogResult{Valid: true, CatalogHash: hash}
		return nil
	})
	return result, err
}
