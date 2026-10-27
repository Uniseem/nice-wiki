package pgdb

import (
	"context"
	"database/sql"
)

// 站点生命周期：每个组件的 schema 里有一张 nw_site_state，记 identity 发来的站点状态（由 sitestate 维护）。
// 没有记录的站点按正常处理；Sites 跳过下架、已删除、已清除的站点。

// RecordSite 记下站点状态；只接受更高的修订号（事件可能重复或乱序）。
func RecordSite(ctx context.Context, db *sql.DB, site, status string, revision int64) error {
	_, err := db.ExecContext(WithSite(ctx, ""), `INSERT INTO nw_site_state(site_id, status, revision) VALUES(?, ?, ?)
		ON CONFLICT(site_id) DO UPDATE SET status=excluded.status, revision=excluded.revision WHERE nw_site_state.revision < excluded.revision`,
		site, status, revision)
	return err
}

// InactiveSites 是记录在案、不在正常状态的站点。
func InactiveSites(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(WithSite(ctx, ""), `SELECT site_id FROM nw_site_state WHERE status <> 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	inactive := map[string]bool{}
	for rows.Next() {
		var site string
		if err := rows.Scan(&site); err != nil {
			return nil, err
		}
		inactive[site] = true
	}
	return inactive, rows.Err()
}

// PurgeSite 在一个事务里删掉本组件所有租户表和带 site_id 列的表中该站点的行（可重复执行）。
// 表之间有外键时先删引用方，再删被引用方。
func PurgeSite(ctx context.Context, db *sql.DB, site string) error {
	ctx = WithSite(ctx, "")
	var schema string
	if err := db.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return err
	}
	columns := map[string]string{} // 租户表 -> 站点列
	rows, err := db.QueryContext(ctx, `SELECT p.tablename, EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema = p.schemaname AND c.table_name = p.tablename AND c.column_name = 'nw_site')
		FROM pg_policies p WHERE p.schemaname = ? AND p.policyname = 'nw_tenant'`, schema)
	if err != nil {
		return err
	}
	for rows.Next() {
		var table string
		var singleton bool
		if err := rows.Scan(&table, &singleton); err != nil {
			rows.Close()
			return err
		}
		columns[table] = "site_id"
		if singleton {
			columns[table] = "nw_site"
		}
	}
	rows.Close()
	// 不做行级隔离的组件（jobs、traffic-guard）按 site_id 列清。
	rows, err = db.QueryContext(ctx, `SELECT table_name FROM information_schema.columns WHERE table_schema = ? AND column_name = 'site_id' AND table_name <> 'nw_site_state'`, schema)
	if err != nil {
		return err
	}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return err
		}
		if _, ok := columns[table]; !ok {
			columns[table] = "site_id"
		}
	}
	rows.Close()
	referencedBy := map[string]map[string]bool{} // 被引用表 -> 引用它的表
	rows, err = db.QueryContext(ctx, `SELECT c.relname, p.relname FROM pg_constraint k
		JOIN pg_class c ON c.oid = k.conrelid JOIN pg_class p ON p.oid = k.confrelid JOIN pg_namespace n ON n.oid = k.connamespace
		WHERE k.contype = 'f' AND n.nspname = ?`, schema)
	if err != nil {
		return err
	}
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			rows.Close()
			return err
		}
		if child != parent {
			if referencedBy[parent] == nil {
				referencedBy[parent] = map[string]bool{}
			}
			referencedBy[parent][child] = true
		}
	}
	rows.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for len(columns) > 0 {
		ready := []string{}
		for table := range columns {
			blocked := false
			for child := range referencedBy[table] {
				if _, pending := columns[child]; pending {
					blocked = true
					break
				}
			}
			if !blocked {
				ready = append(ready, table)
			}
		}
		if len(ready) == 0 { // 外键成环时没有可先删的表，剩下的一起删，避免死循环
			for table := range columns {
				ready = append(ready, table)
			}
		}
		for _, table := range ready {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+quoteIdent(schema)+`.`+quoteIdent(table)+` WHERE `+quoteIdent(columns[table])+` = ?`, site); err != nil {
				return err
			}
			delete(columns, table)
		}
	}
	return tx.Commit()
}

// siteStateSQL 建本 schema 的站点状态表（建 schema 时调用，不经方言翻译）。
func siteStateSQL(schema string) string {
	return `CREATE TABLE IF NOT EXISTS ` + quoteIdent(schema) + `.nw_site_state(site_id text PRIMARY KEY, status text NOT NULL, revision bigint NOT NULL)`
}
