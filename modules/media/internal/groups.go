package media

import (
	"context"
	"strings"
	"unicode/utf8"

	"nice-wiki/libs/platform"
)

type AssetGroup struct {
	GroupID    string `json:"groupId"`
	Name       string `json:"name"`
	AssetCount int64  `json:"assetCount"`
}
type ListAssetGroupsInput struct{}
type ListAssetGroupsResult struct {
	Groups []AssetGroup `json:"groups"`
}
type CreateAssetGroupInput struct {
	Name string `json:"name"`
}
type CreateAssetGroupResult struct {
	Group AssetGroup `json:"group"`
}
type DeleteAssetGroupInput struct {
	GroupID string `json:"groupId"`
}
type DeleteAssetGroupResult struct {
	Deleted bool `json:"deleted"`
}
type SetAssetGroupInput struct {
	AssetID string `json:"assetId"`
	GroupID string `json:"groupId"`
}
type SetAssetGroupResult struct {
	Updated bool `json:"updated"`
}

func ListAssetGroups(ctx context.Context, op OperationContext, input ListAssetGroupsInput) (ListAssetGroupsResult, error) {
	result := ListAssetGroupsResult{Groups: []AssetGroup{}}
	s, err := assetListingStore(op)
	if err != nil {
		return result, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT g.group_id,g.name,(SELECT COUNT(*) FROM assets a WHERE a.site_id=g.site_id AND a.state<>'deleted' AND json_extract(a.asset_json,'$.groupId')=g.group_id) FROM asset_groups g WHERE g.site_id=? ORDER BY g.created_at,g.group_id LIMIT 200`, op.Actor.SiteID)
	if err != nil {
		return result, mapMediaDBError(err, "读取资源分组失败")
	}
	defer rows.Close()
	for rows.Next() {
		var g AssetGroup
		if err = rows.Scan(&g.GroupID, &g.Name, &g.AssetCount); err != nil {
			return result, mapMediaDBError(err, "读取分组失败")
		}
		result.Groups = append(result.Groups, g)
	}
	return result, mapMediaDBError(rows.Err(), "读取分组失败")
}

func CreateAssetGroup(ctx context.Context, op OperationContext, input CreateAssetGroupInput) (CreateAssetGroupResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return CreateAssetGroupResult{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 100 {
		return CreateAssetGroupResult{}, platform.NewError(platform.CodeValidation, false, "分组名称须为1至100字")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return CreateAssetGroupResult{}, err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return CreateAssetGroupResult{}, mapMediaDBError(err, "开始分组事务失败")
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(work, `SELECT COUNT(*) FROM asset_groups WHERE site_id=?`, op.Actor.SiteID).Scan(&count); err != nil {
		return CreateAssetGroupResult{}, mapMediaDBError(err, "读取分组数量失败")
	}
	if count >= 200 {
		return CreateAssetGroupResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "最多200个资源分组")
	}
	g := AssetGroup{GroupID: platform.NewIdentifier(), Name: input.Name}
	if _, err = tx.ExecContext(work, `INSERT INTO asset_groups VALUES(?,?,?,?) ON CONFLICT(site_id,name) DO NOTHING`, g.GroupID, op.Actor.SiteID, g.Name, platform.Now()); err != nil {
		return CreateAssetGroupResult{}, mapMediaDBError(err, "创建资源分组失败")
	}
	if err = tx.QueryRowContext(work, `SELECT group_id FROM asset_groups WHERE site_id=? AND name=?`, op.Actor.SiteID, g.Name).Scan(&g.GroupID); err != nil {
		return CreateAssetGroupResult{}, mapMediaDBError(err, "读取新分组失败")
	}
	if err = tx.Commit(); err != nil {
		return CreateAssetGroupResult{}, mapMediaDBError(err, "提交分组失败")
	}
	return CreateAssetGroupResult{Group: g}, nil
}

func DeleteAssetGroup(ctx context.Context, op OperationContext, input DeleteAssetGroupInput) (DeleteAssetGroupResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return DeleteAssetGroupResult{}, err
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return DeleteAssetGroupResult{}, err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return DeleteAssetGroupResult{}, mapMediaDBError(err, "开始删除分组失败")
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(work, `UPDATE assets SET asset_json=json_remove(asset_json,'$.groupId') WHERE site_id=? AND json_extract(asset_json,'$.groupId')=?`, op.Actor.SiteID, input.GroupID); err != nil {
		return DeleteAssetGroupResult{}, mapMediaDBError(err, "移出分组失败")
	}
	if _, err = tx.ExecContext(work, `DELETE FROM asset_groups WHERE site_id=? AND group_id=?`, op.Actor.SiteID, input.GroupID); err != nil {
		return DeleteAssetGroupResult{}, mapMediaDBError(err, "删除分组失败")
	}
	if err = tx.Commit(); err != nil {
		return DeleteAssetGroupResult{}, mapMediaDBError(err, "提交删除分组失败")
	}
	return DeleteAssetGroupResult{Deleted: true}, nil
}

func SetAssetGroup(ctx context.Context, op OperationContext, input SetAssetGroupInput) (SetAssetGroupResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return SetAssetGroupResult{}, err
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return SetAssetGroupResult{}, err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return SetAssetGroupResult{}, mapMediaDBError(err, "开始移动资源失败")
	}
	defer tx.Rollback()
	if input.GroupID != "" {
		var found int
		if err = tx.QueryRowContext(work, `SELECT COUNT(*) FROM asset_groups WHERE site_id=? AND group_id=?`, op.Actor.SiteID, input.GroupID).Scan(&found); err != nil {
			return SetAssetGroupResult{}, mapMediaDBError(err, "读取目标分组失败")
		}
		if found == 0 {
			return SetAssetGroupResult{}, platform.NewError(platform.CodeNotFound, false, "目标分组不存在")
		}
	}
	result, err := tx.ExecContext(work, `UPDATE assets SET asset_json=CASE WHEN ?='' THEN json_remove(asset_json,'$.groupId') ELSE json_set(asset_json,'$.groupId',?) END WHERE site_id=? AND asset_id=? AND state='ready'`, input.GroupID, input.GroupID, op.Actor.SiteID, input.AssetID)
	if err != nil {
		return SetAssetGroupResult{}, mapMediaDBError(err, "移动资源失败")
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return SetAssetGroupResult{}, platform.NewError(platform.CodeNotFound, false, "资源不存在或不可移动")
	}
	if err = tx.Commit(); err != nil {
		return SetAssetGroupResult{}, mapMediaDBError(err, "提交移动资源失败")
	}
	return SetAssetGroupResult{Updated: true}, nil
}
