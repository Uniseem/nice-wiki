package pgdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// 租户隔离：多个站点共用同一批表，靠 PostgreSQL 行级安全（RLS）隔离。
//
//   - 驱动按请求上下文里的站点设置会话变量 nw.site；没有站点（后台循环、启动）时为空，表示系统视角，能看到全部行；
//     WithTemplateView 再设置 nw.tpl，系统视角就只剩模板行（组件启动、就绪检查读自己的状态用）。
//   - 建表时给每张租户表加站点列并启用 RLS（FORCE，表属主也受限）；策略是：会话站点为空，或行的站点列等于会话站点。
//   - 只有一行的“单例”表（singleton=1）改成每站点一行：新增列 nw_site，主键改为 (nw_site, singleton)；
//     站点第一次出现时由驱动把系统视角（nw_site=''）的模板行复制一份给该站点。
//   - 自然键会在站点间重复的表（siteKeyTables）把 site_id 并入主键和唯一约束。
//
// 连接必须用非超级用户、无 BYPASSRLS 的数据库角色，否则 RLS 不生效。

// tenantModules 是启用租户隔离的组件。
var tenantModules = map[string]bool{
	"content": true, "media": true, "localization": true, "knowledge": true, "qa": true, "memory": true, "analytics": true, "site-config": true,
}

// globalTables 是租户组件里不做隔离的表（迁移记录、运行元数据、事件收发件箱、已废弃的快照机制等）。
var globalTables = map[string]bool{
	"schema_migrations": true, "nw_pragma": true, "nw_outbox": true, "nw_outbox_control": true, "nw_inbox": true,
	"runtime_meta": true, "runtime_state": true, "local_barrier": true, "barriers": true, "reference_scan_state": true,
	"domain_versions": true, "control_state": true, "restore_receipts": true, "control_schema": true,
}

// siteKeyTables 是自然键在站点间可能重复的表：主键与唯一约束并入 site_id。
var siteKeyTables = map[string]bool{
	"daily_usage": true, "aggregation_watermarks": true, "qa_turn_counts": true, "aggregate_runs": true, "beacon_period_watermarks": true,
	"aggregate_jobs": true, "languages": true, "interface_catalog": true, "interface_sources": true, "audit_anchors": true,
}

var singletons sync.Map // "<模块>.<表>" -> true

func isTenant(module string) bool { return tenantModules[module] }

const siteDefault = `COALESCE(current_setting('nw.site', true), '')`

func policyExpr(column string) string {
	return `((NULLIF(current_setting('nw.site', true), '') IS NULL AND COALESCE(current_setting('nw.tpl', true), '') = '') OR "` + column + `" = current_setting('nw.site', true))`
}

func isConstraintItem(item []token) bool {
	item = trimSpace(item)
	if len(item) == 0 {
		return true
	}
	switch strings.ToUpper(item[0].text) {
	case "PRIMARY", "UNIQUE", "FOREIGN", "CHECK", "CONSTRAINT":
		return item[0].kind == tkWord
	}
	return false
}

func columnName(item []token) string {
	item = trimSpace(item)
	if len(item) == 0 {
		return ""
	}
	return strings.ToLower(strings.Trim(item[0].text, `"`))
}

func hasSeq(item []token, a, b string) int {
	for i := 0; i < len(item); i++ {
		if item[i].is(a) {
			if j := nextSig(item, i); j < len(item) && item[j].is(b) {
				return i
			}
		}
	}
	return -1
}

// TenantScript 先做 SQLite→PostgreSQL 翻译，再对租户组件做隔离相关的改写。
func TenantScript(module, sql string) ([]string, error) {
	statements, err := TranslateScript(sql)
	if err != nil || !isTenant(module) {
		return statements, err
	}
	var out []string
	for _, statement := range statements {
		upper := strings.ToUpper(strings.TrimSpace(statement))
		switch {
		case strings.HasPrefix(upper, "CREATE TABLE"):
			rewritten, err := tenantizeTable(module, statement)
			if err != nil {
				return nil, err
			}
			out = append(out, rewritten...)
		case strings.HasPrefix(upper, "CREATE UNIQUE INDEX"):
			out = append(out, tenantizeIndex(statement))
		case strings.HasPrefix(upper, "INSERT"):
			out = append(out, tenantDML(module, statement))
		default:
			out = append(out, statement)
		}
	}
	return out, nil
}

// tenantizeTable 改写一条 CREATE TABLE，并追加启用 RLS 的语句。
func tenantizeTable(module, statement string) ([]string, error) {
	tokens := tokenize(statement)
	tableAt := indexOfWord(tokens, "TABLE")
	if tableAt < 0 {
		return []string{statement}, nil
	}
	nameAt := nextSig(tokens, tableAt)
	if nameAt < len(tokens) && tokens[nameAt].is("IF") {
		nameAt = nextSig(tokens, nextSig(tokens, nextSig(tokens, nameAt)))
	}
	if nameAt >= len(tokens) {
		return []string{statement}, nil
	}
	table := strings.Trim(tokens[nameAt].text, `"`)
	if globalTables[strings.ToLower(table)] {
		return []string{statement}, nil
	}
	open := nextSig(tokens, nameAt)
	if open >= len(tokens) || tokens[open].text != "(" {
		return []string{statement}, nil
	}
	closeAt := matchParen(tokens, open)
	if closeAt < 0 {
		return nil, fmt.Errorf("pgdb: 建表语句括号不匹配")
	}
	items := splitArgs(tokens, open, closeAt)
	hasSite, singleton := false, false
	for _, item := range items {
		if isConstraintItem(item) {
			continue
		}
		switch columnName(item) {
		case "site_id":
			hasSite = true
		case "singleton":
			singleton = true
		}
	}
	policyColumn := "site_id"
	var extra []string
	switch {
	case singleton:
		policyColumn = "nw_site"
		items = stripColumnKey(items, "singleton", "PRIMARY")
		extra = append(extra, "nw_site TEXT NOT NULL DEFAULT "+siteDefault, "PRIMARY KEY (nw_site, singleton)")
		singletons.Store(module+"."+strings.ToLower(table), true)
	case siteKeyTables[strings.ToLower(table)]:
		if !hasSite {
			extra = append(extra, "site_id TEXT NOT NULL DEFAULT "+siteDefault)
		}
		items = prependSiteToKeys(items)
	case !hasSite:
		extra = append(extra, "site_id TEXT NOT NULL DEFAULT "+siteDefault)
	}
	var body []string
	for _, item := range items {
		body = append(body, strings.TrimSpace(render(item)))
	}
	body = append(body, extra...)
	create := render(tokens[:open+1]) + "\n  " + strings.Join(body, ",\n  ") + "\n)" + render(tokens[closeAt+1:])
	quoted := `"` + table + `"`
	return []string{
		create,
		"ALTER TABLE " + quoted + " ENABLE ROW LEVEL SECURITY",
		"ALTER TABLE " + quoted + " FORCE ROW LEVEL SECURITY",
		"DROP POLICY IF EXISTS nw_tenant ON " + quoted,
		"CREATE POLICY nw_tenant ON " + quoted + " USING " + policyExpr(policyColumn) + " WITH CHECK " + policyExpr(policyColumn),
	}, nil
}

// stripColumnKey 去掉某列定义里的“PRIMARY KEY”。
func stripColumnKey(items [][]token, column, word string) [][]token {
	out := make([][]token, len(items))
	for i, item := range items {
		if !isConstraintItem(item) && columnName(item) == column {
			if at := hasSeq(item, word, "KEY"); at >= 0 {
				end := nextSig(item, at)
				item = append(append([]token{}, item[:at]...), item[end+1:]...)
			}
		}
		out[i] = item
	}
	return out
}

// prependSiteToKeys 把 site_id 并入主键和唯一约束（表级和列级）。
func prependSiteToKeys(items [][]token) [][]token {
	var out [][]token
	var added []string
	for _, item := range items {
		if isConstraintItem(item) {
			text := strings.TrimSpace(render(item))
			upper := strings.ToUpper(text)
			if (strings.HasPrefix(upper, "PRIMARY KEY") || strings.HasPrefix(upper, "UNIQUE")) && !strings.Contains(strings.ToLower(text), "site_id") {
				if open := strings.Index(text, "("); open >= 0 {
					text = text[:open+1] + "site_id, " + text[open+1:]
					out = append(out, tokenize(text))
					continue
				}
			}
			out = append(out, item)
			continue
		}
		name := columnName(item)
		for _, key := range []string{"PRIMARY", "UNIQUE"} {
			var at int
			if key == "PRIMARY" {
				at = hasSeq(item, "PRIMARY", "KEY")
			} else {
				at = -1
				for i, t := range item {
					if t.is("UNIQUE") {
						at = i
						break
					}
				}
			}
			if at < 0 {
				continue
			}
			end := at
			if key == "PRIMARY" {
				end = nextSig(item, at)
			}
			item = append(append([]token{}, item[:at]...), item[end+1:]...)
			added = append(added, key+"|"+name)
		}
		out = append(out, item)
	}
	for _, a := range added {
		parts := strings.SplitN(a, "|", 2)
		if parts[0] == "PRIMARY" {
			out = append(out, tokenize("PRIMARY KEY (site_id, "+parts[1]+")"))
		} else {
			out = append(out, tokenize("UNIQUE (site_id, "+parts[1]+")"))
		}
	}
	return out
}

// tenantizeIndex 把自然键表上的唯一索引并入 site_id。
func tenantizeIndex(statement string) string {
	tokens := tokenize(statement)
	on := indexOfWord(tokens, "ON")
	if on < 0 {
		return statement
	}
	t := nextSig(tokens, on)
	if t >= len(tokens) || !siteKeyTables[strings.ToLower(strings.Trim(tokens[t].text, `"`))] {
		return statement
	}
	open := nextSig(tokens, t)
	if open >= len(tokens) || tokens[open].text != "(" {
		return statement
	}
	return render(tokens[:open+1]) + "site_id, " + render(tokens[open+1:])
}

// tenantDML 改写 INSERT … ON CONFLICT：单例表的冲突目标 (singleton) 改成 (nw_site, singleton)，
// 自然键表的冲突目标前面并入 site_id。
func tenantDML(module, statement string) string {
	tokens := tokenize(statement)
	into := indexOfWord(tokens, "INTO")
	if into < 0 {
		return statement
	}
	t := nextSig(tokens, into)
	if t >= len(tokens) {
		return statement
	}
	table := strings.ToLower(strings.Trim(tokens[t].text, `"`))
	_, isSingleton := singletons.Load(module + "." + table)
	if !isSingleton && !siteKeyTables[table] {
		return statement
	}
	conflict := indexOfWord(tokens, "CONFLICT")
	if conflict < 0 {
		return statement
	}
	open := nextSig(tokens, conflict)
	if open >= len(tokens) || tokens[open].text != "(" {
		return statement
	}
	closeAt := matchParen(tokens, open)
	inner := strings.TrimSpace(render(tokens[open+1 : closeAt]))
	switch {
	case isSingleton && strings.EqualFold(inner, "singleton"):
		inner = "nw_site, singleton"
	case siteKeyTables[table] && !strings.Contains(strings.ToLower(inner), "site_id"):
		inner = "site_id, " + inner
	default:
		return statement
	}
	return render(tokens[:open+1]) + inner + render(tokens[closeAt:])
}

// Sites 列出本组件里出现过的站点，后台循环按它逐站点处理。默认取各单例表里 nw_site 的并集（不含模板行）；
// 组件没有单例表时，传入带 site_id 列的表名（如 languages）。站点第一次被访问时才会出现，结果缓存几秒。
func Sites(ctx context.Context, db *sql.DB, tables ...string) ([]string, error) {
	var schema string
	if err := db.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return nil, err
	}
	cacheKey := schema + "\x00" + strings.Join(tables, ",")
	sitesMu.Lock()
	cached, ok := sitesCache[cacheKey]
	sitesMu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached.sites, nil
	}
	var parts []string
	if len(tables) > 0 {
		for _, table := range tables {
			parts = append(parts, `SELECT site_id AS nw_site FROM `+quoteIdent(schema)+`.`+quoteIdent(table))
		}
	} else {
		rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.columns WHERE table_schema = current_schema() AND column_name = 'nw_site'`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var table string
			if rows.Scan(&table) == nil {
				parts = append(parts, `SELECT nw_site FROM `+quoteIdent(schema)+`.`+quoteIdent(table))
			}
		}
		rows.Close()
	}
	var sites []string
	if len(parts) > 0 {
		// 系统视角（不带站点、不带模板视角）才能看到全部行。
		conn, err := db.Conn(WithSite(ctx, ""))
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		result, err := conn.QueryContext(WithSite(ctx, ""), `SELECT DISTINCT nw_site FROM (`+strings.Join(parts, " UNION ALL ")+`) s WHERE nw_site <> '' ORDER BY nw_site`)
		if err != nil {
			return nil, err
		}
		defer result.Close()
		for result.Next() {
			var site string
			if result.Scan(&site) == nil {
				sites = append(sites, site)
			}
		}
	}
	sitesMu.Lock()
	sitesCache[cacheKey] = sitesEntry{sites, time.Now().Add(3 * time.Second)}
	sitesMu.Unlock()
	return sites, nil
}

type sitesEntry struct {
	sites   []string
	expires time.Time
}

var (
	sitesMu    sync.Mutex
	sitesCache = map[string]sitesEntry{}
)
