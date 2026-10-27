// Package pgdb 是各组件接入共用 PostgreSQL 的兼容层：组件里原有的 sql.Open("sqlite", …) 调用点不动，
// 这里注册同名驱动，把“<数据目录>/active/<库>.db”映射到 PostgreSQL 的独立 schema，
// 并在执行时把 SQLite 方言的语句改写为 PostgreSQL（占位符、函数、DDL、触发器）。
//
// 写事务开头取本 schema 的建议锁，保持原先 SQLite 单写者的串行语义；热路径需要并发时再单独去掉。
package pgdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"nice-wiki/libs/platform"
)

func init() { sql.Register("sqlite", &Driver{}) }

type component struct{ module, dataRoot, dsn string }

var (
	registryMu sync.RWMutex
	registry   = map[string]component{}
)

// Configure 登记组件的数据目录与连接串，须在该组件打开任何库之前调用。
func Configure(module, dataRoot, dsn string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[module] = component{module: module, dataRoot: filepath.Clean(dataRoot), dsn: dsn}
}

// ReadDSN 读取连接串引用文件；路径为空表示未配置。
func ReadDSN(ref string) (string, error) {
	if ref == "" {
		return "", nil
	}
	raw, err := os.ReadFile(ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

// Driver 是名为 "sqlite" 的兼容驱动。
type Driver struct{}

func (Driver) Open(name string) (driver.Conn, error) {
	connector, err := Driver{}.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return connector.Connect(context.Background())
}

func sanitizeSchema(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return '_'
	}, name)
}

// resolve 把旧的 SQLite DSN 映射到 (连接串, schema, 占位文件路径)。
func resolve(legacy string) (component, string, string, error) {
	path := strings.TrimPrefix(legacy, "file:")
	if q := strings.IndexByte(path, '?'); q >= 0 {
		path = path[:q]
	}
	if unescaped, err := url.PathUnescape(path); err == nil {
		path = unescaped
	}
	path = filepath.Clean(path)
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, c := range registry {
		rel, err := filepath.Rel(c.dataRoot, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 2 || (parts[0] != "active" && parts[0] != "control") || !strings.HasSuffix(parts[1], ".db") {
			continue
		}
		base := strings.TrimSuffix(parts[1], ".db")
		if base == "control" {
			base = c.module + "_control"
		}
		return c, sanitizeSchema(base), path, nil
	}
	return component{}, "", "", fmt.Errorf("pgdb: %s 不在任何已登记组件的 active/control 目录下（PostgreSQL 下不支持快照副本库）", path)
}

// helperSQL 是翻译后的语句会调用的辅助函数，放在 public schema。
var helperSQL = []string{
	`CREATE OR REPLACE FUNCTION public.nw_json_type(j jsonb) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT CASE jsonb_typeof(j) WHEN 'string' THEN 'text' WHEN 'number' THEN CASE WHEN (j #>> '{}')::numeric = trunc((j #>> '{}')::numeric) THEN 'integer' ELSE 'real' END WHEN 'boolean' THEN (j #>> '{}') ELSE jsonb_typeof(j) END $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_value(j jsonb) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT CASE WHEN jsonb_typeof(j) IN ('array','object') THEN j::text ELSE j #>> '{}' END $$`,
	`CREATE OR REPLACE FUNCTION public.nw_raise(msg text) RETURNS bigint LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '%', msg; END $$`,
	`CREATE OR REPLACE FUNCTION public.nw_text(b bytea) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT convert_from(b, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_text(v anyelement) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT v::text $$`,
	`CREATE OR REPLACE FUNCTION public.nw_bytes(t text) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT convert_to(t, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_bytes(b bytea) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT b $$`,
	`CREATE OR REPLACE FUNCTION public.nw_jsonb(t text) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT CASE WHEN t IS NULL OR NOT pg_input_is_valid(t, 'jsonb') THEN NULL ELSE t::jsonb END $$`,
	`CREATE OR REPLACE FUNCTION public.nw_jsonb(b bytea) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT public.nw_jsonb(convert_from(b, 'UTF8')) $$`,
	`CREATE OR REPLACE FUNCTION public.nw_jsonb(j jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT j $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_set(x text, p text[], v jsonb) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT jsonb_set(public.nw_jsonb(x), p, v)::text $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_set(x bytea, p text[], v jsonb) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT convert_to(jsonb_set(public.nw_jsonb(x), p, v)::text, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_remove(x text, p text[]) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT (public.nw_jsonb(x) #- p)::text $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_remove(x bytea, p text[]) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT convert_to((public.nw_jsonb(x) #- p)::text, 'UTF8') $$`,
}

// operatorSQL 补上 SQLite 宽松比较在 PostgreSQL 里缺的运算符：文本与整数、布尔与整数。
// json_extract 翻译后是文本，而代码拿它跟整数参数比较；json_valid(x)=0 这类也一样。
func operatorSQL() []string {
	type op struct{ sym, name, numeric, otherwise string }
	// 文本在左、整数在右：文本是数字就按数值比较，否则按 SQLite 的类型顺序（整数 < 文本）。
	textLeft := []op{{"=", "eq", "a::numeric = b", "false"}, {"<>", "ne", "a::numeric <> b", "true"}, {"<", "lt", "a::numeric < b", "false"},
		{"<=", "le", "a::numeric <= b", "false"}, {">", "gt", "a::numeric > b", "true"}, {">=", "ge", "a::numeric >= b", "true"}}
	intLeft := []op{{"=", "eq", "a = b::numeric", "false"}, {"<>", "ne", "a <> b::numeric", "true"}, {"<", "lt", "a < b::numeric", "true"},
		{"<=", "le", "a <= b::numeric", "true"}, {">", "gt", "a > b::numeric", "false"}, {">=", "ge", "a >= b::numeric", "false"}}
	var out []string
	add := func(left, right, prefix string, o op, cond, body string) {
		fn := "public.nw_" + prefix + "_" + o.name
		out = append(out, "CREATE OR REPLACE FUNCTION "+fn+"(a "+left+", b "+right+") RETURNS boolean LANGUAGE sql IMMUTABLE AS $$ SELECT "+body+" $$")
		out = append(out, "DO $$ BEGIN CREATE OPERATOR public."+o.sym+" (LEFTARG="+left+", RIGHTARG="+right+", FUNCTION="+fn+"); EXCEPTION WHEN duplicate_function THEN NULL; END $$")
	}
	for _, o := range textLeft {
		add("text", "bigint", "tb", o, "", "CASE WHEN a ~ '^-?[0-9]+$' THEN "+o.numeric+" ELSE "+o.otherwise+" END")
	}
	for _, o := range intLeft {
		add("bigint", "text", "bt", o, "", "CASE WHEN b ~ '^-?[0-9]+$' THEN "+o.numeric+" ELSE "+o.otherwise+" END")
	}
	add("boolean", "bigint", "ob", op{"=", "eq", "", ""}, "", "a = (b <> 0)")
	add("boolean", "bigint", "ob", op{"<>", "ne", "", ""}, "", "a <> (b <> 0)")
	add("bigint", "boolean", "bo", op{"=", "eq", "", ""}, "", "(a <> 0) = b")
	add("bigint", "boolean", "bo", op{"<>", "ne", "", ""}, "", "(a <> 0) <> b")
	return out
}

func (d Driver) OpenConnector(name string) (driver.Connector, error) {
	connector, err := d.openConnector(name)
	if err != nil {
		debugf("open", err, name)
	}
	return connector, err
}

func (Driver) openConnector(name string) (driver.Connector, error) {
	c, schema, marker, err := resolve(name)
	if err != nil {
		return nil, err
	}
	// 组件启动时会校验库文件路径是正规私有文件；PostgreSQL 下放一个空占位文件满足这类检查。
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return nil, fmt.Errorf("pgdb: 数据目录不可用")
	}
	if file, err := os.OpenFile(marker, os.O_CREATE|os.O_RDWR, 0o600); err == nil {
		_ = file.Close()
	}
	if c.dsn == "" {
		return nil, fmt.Errorf("pgdb: 组件 %s 未配置 PostgreSQL 连接串（databaseRef）", c.module)
	}
	config, err := pgx.ParseConfig(c.dsn)
	if err != nil {
		return nil, fmt.Errorf("pgdb: 连接串不合法")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ensureSchema(ctx, config, schema); err != nil {
		return nil, err
	}
	loadTables(ctx, config, c.module, schema)
	config = config.Copy()
	config.RuntimeParams["search_path"] = quoteIdent(schema) + ",public"
	config.RuntimeParams["application_name"] = "nw-" + c.module
	hash := fnv.New64a()
	hash.Write([]byte(schema))
	return &connector{inner: stdlib.GetConnector(*config), config: config, lockKey: int64(hash.Sum64() >> 1), module: c.module, schema: schema, tenant: isTenant(c.module)}, nil
}

// InstallHelpers 在 public schema 安装翻译后语句依赖的辅助函数。
func InstallHelpers(ctx context.Context, conn *pgx.Conn) error {
	for _, statement := range append(append([]string{}, helperSQL...), operatorSQL()...) {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("pgdb: 创建辅助函数失败：%w", err)
		}
	}
	return nil
}

func quoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func ensureSchema(ctx context.Context, config *pgx.ConnConfig, schema string) error {
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("pgdb: 连接 PostgreSQL 失败：%w", err)
	}
	defer conn.Close(ctx)
	// 多个组件同时启动时串行化建 schema 与辅助函数，避免并发建同名对象。
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(7242001)"); err != nil {
		return err
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock(7242001)")
	if err := InstallHelpers(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+quoteIdent(schema)); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+quoteIdent(schema)+".nw_pragma(name text PRIMARY KEY, value bigint NOT NULL)"); err != nil {
		return err
	}
	return upgradePolicies(ctx, conn, schema)
}

type connector struct {
	inner       driver.Connector
	config      *pgx.ConnConfig // 复制单例模板行用独立连接，不受调用方事务（可能是只读的）影响
	lockKey     int64
	module      string
	schema      string
	tenant      bool
	provMu      sync.Mutex
	provisioned sync.Map // 站点 -> 已在本组件复制过单例模板行
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	inner, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &conn{Conn: inner, owner: c, described: map[string][]uint32{}}, nil
}

func (c *connector) Driver() driver.Driver { return Driver{} }

// conn 包装 pgx 的 database/sql 连接：翻译语句、规整参数、写事务取建议锁。
type conn struct {
	driver.Conn
	owner     *connector
	inTx      bool
	site      string   // 会话变量 nw.site 当前值
	mode      viewMode // 会话变量对应的视角
	siteSet   bool
	described map[string][]uint32 // 语句文本 -> 服务端推断的参数类型
}

// txWrapper 在提交或回滚后清掉连接的“事务中”标记。
type txWrapper struct {
	driver.Tx
	c *conn
}

func (t *txWrapper) Commit() error { t.c.inTx = false; return t.Tx.Commit() }
func (t *txWrapper) Rollback() error {
	t.c.inTx = false
	t.c.siteSet = false // 回滚会撤销事务内设置的会话变量，下次重新设置
	return t.Tx.Rollback()
}

var cache sync.Map // 模块+SQLite 语句文本 -> []string

func translate(module, query string) ([]string, error) {
	key := module + "\x00" + query
	if cached, ok := cache.Load(key); ok {
		return cached.([]string), nil
	}
	translated, err := TenantScript(module, query)
	if err != nil {
		return nil, err
	}
	if len(translated) == 0 {
		translated = []string{"SELECT 1"}
	}
	cache.Store(key, translated)
	return translated, nil
}

type viewKey int

const (
	siteOverrideKey viewKey = iota
	templateViewKey
)

// WithSite 指定访问租户表的站点（默认取请求上下文里的站点）。site 为空表示系统视角：看得到全部站点，
// 只给确实要跨站点工作的后台代码用（列出站点、迁移、跨站点汇总）。
func WithSite(ctx context.Context, site string) context.Context {
	return context.WithValue(ctx, siteOverrideKey, site)
}

// WithTemplateView 让本次访问只看到单例表的模板行（nw_site=”）：启动、就绪检查把“组件自己的状态”当成单站点读取。
func WithTemplateView(ctx context.Context) context.Context {
	return context.WithValue(ctx, templateViewKey, true)
}

// viewMode 是访问租户表的视角。
type viewMode int

const (
	viewMissing  viewMode = iota // 上下文没给站点：租户表读不到也写不进
	viewSystem                   // WithSite(ctx, "")：全部站点
	viewTemplate                 // 只看单例表的模板行
	viewSite                     // 某个站点
)

// viewOf 取访问视角，以及上下文有没有给出视角信息。事务里某条语句用了不带任何站点信息的上下文
// （比如入队事件用的 Background），沿用事务开始时的视角，不要把会话站点清掉。
func viewOf(ctx context.Context) (mode viewMode, site string, given bool) {
	template, _ := ctx.Value(templateViewKey).(bool)
	if site, ok := ctx.Value(siteOverrideKey).(string); ok {
		switch {
		case site != "":
			return viewSite, site, true
		case template:
			return viewTemplate, "", true
		}
		return viewSystem, "", true
	}
	op, ok := platform.OperationContextFrom(ctx)
	switch {
	case ok && op.Actor.SiteID != "":
		return viewSite, string(op.Actor.SiteID), true
	case template:
		return viewTemplate, "", true
	}
	return viewMissing, "", ok
}

// useSite 让本连接的会话变量（nw.site、nw.tpl、nw.sys）与访问视角一致；站点第一次出现时先复制单例模板行。
func (c *conn) useSite(ctx context.Context, statement string) error {
	if !c.owner.tenant {
		return nil
	}
	mode, site, given := viewOf(ctx)
	if !given && c.inTx && c.siteSet {
		mode, site = c.mode, c.site
	}
	if mode == viewMissing && touchesTenant(c.owner.module, statement) {
		c.owner.noteMissingSite(statement)
	}
	if c.siteSet && c.site == site && c.mode == mode {
		return nil
	}
	execer := c.Conn.(driver.ExecerContext)
	if mode == viewSite {
		if err := c.owner.ensureProvisioned(ctx, c, site); err != nil {
			return err
		}
	}
	// 没有站点信息（viewMissing）时三个变量都为空，租户表读不到也写不进。
	var tpl, sys string
	switch mode {
	case viewTemplate:
		tpl = "on"
	case viewSystem:
		sys = "on"
	}
	if _, err := execer.ExecContext(ctx, "SELECT set_config('nw.site', $1, false), set_config('nw.tpl', $2, false), set_config('nw.sys', $3, false)",
		[]driver.NamedValue{{Ordinal: 1, Value: site}, {Ordinal: 2, Value: tpl}, {Ordinal: 3, Value: sys}}); err != nil {
		return err
	}
	c.site, c.mode, c.siteSet = site, mode, true
	return nil
}

var touchCache sync.Map // 组件+语句 -> 是否引用租户表

// touchesTenant 判断语句是否引用了本组件的租户表；建表、改表这类结构语句不受行级安全影响，不算。
func touchesTenant(module, statement string) bool {
	key := module + "\x00" + statement
	if cached, ok := touchCache.Load(key); ok {
		return cached.(bool)
	}
	tokens := tokenize(statement)
	found := false
	if first := nextSig(tokens, -1); first < len(tokens) && !tokens[first].is("CREATE") && !tokens[first].is("ALTER") && !tokens[first].is("DROP") {
		for _, t := range tokens {
			if t.kind != tkWord && t.kind != tkQuoted {
				continue
			}
			if _, ok := tenantTables.Load(module + "." + strings.ToLower(strings.Trim(t.text, `"`))); ok {
				found = true
				break
			}
		}
	}
	touchCache.Store(key, found)
	return found
}

var missingSeen sync.Map // 组件+调用位置 -> struct{}

// noteMissingSite 记下没带站点信息就访问租户表的调用位置（这类访问被行级安全拒绝），每个位置只记一次；
// 语句只取开头，不含参数。
func (o *connector) noteMissingSite(statement string) {
	where := callers()
	if _, seen := missingSeen.LoadOrStore(o.module+"\x00"+where, struct{}{}); seen {
		return
	}
	statement = strings.Join(strings.Fields(statement), " ")
	if len(statement) > 120 {
		statement = statement[:120] + "…"
	}
	fmt.Fprintf(os.Stderr, "pgdb[%s] 租户表访问没有站点信息，已拒绝：%s | %s\n", o.module, where, statement)
}

// callers 取调用栈里 database/sql 与本包之外的前三层。
func callers() string {
	pcs := make([]uintptr, 48)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs)])
	var out []string
	for len(out) < 3 {
		frame, more := frames.Next()
		if frame.Function != "" && !strings.HasPrefix(frame.Function, "database/sql.") && !strings.HasPrefix(frame.Function, "nice-wiki/libs/platform/pgdb.") {
			out = append(out, strings.TrimPrefix(frame.Function, "nice-wiki/")+":"+strconv.Itoa(frame.Line))
		}
		if !more {
			break
		}
	}
	return strings.Join(out, " < ")
}

// ensureProvisioned 在系统视角下，把本组件各单例表里 nw_site=” 的模板行复制给新站点（幂等）。
func (o *connector) ensureProvisioned(ctx context.Context, c *conn, site string) error {
	if _, ok := o.provisioned.Load(site); ok {
		return nil
	}
	o.provMu.Lock()
	defer o.provMu.Unlock()
	if _, ok := o.provisioned.Load(site); ok {
		return nil
	}
	pgxConn, err := pgx.ConnectConfig(ctx, o.config)
	if err != nil {
		return err
	}
	defer pgxConn.Close(context.Background())
	if _, err := pgxConn.Exec(ctx, "SELECT set_config('nw.sys', 'on', false)"); err != nil {
		return err
	}
	rows, err := pgxConn.Query(ctx, `SELECT table_name, column_name FROM information_schema.columns WHERE table_schema = $1 AND table_name IN (SELECT table_name FROM information_schema.columns WHERE table_schema = $1 AND column_name = 'nw_site') ORDER BY table_name, ordinal_position`, o.schema)
	if err != nil {
		return err
	}
	columns := map[string][]string{}
	var tables []string
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			rows.Close()
			return err
		}
		if _, ok := columns[table]; !ok {
			tables = append(tables, table)
		}
		columns[table] = append(columns[table], column)
	}
	rows.Close()
	for _, table := range tables {
		var list, values []string
		for _, column := range columns[table] {
			list = append(list, quoteIdent(column))
			switch column {
			case "nw_site":
				values = append(values, "$1")
			case "site_id":
				values = append(values, `CASE WHEN "site_id" IS NULL THEN NULL ELSE $1 END`)
			default:
				values = append(values, quoteIdent(column))
			}
		}
		name := quoteIdent(o.schema) + "." + quoteIdent(table)
		if _, err := pgxConn.Exec(ctx, "INSERT INTO "+name+" ("+strings.Join(list, ",")+") SELECT "+strings.Join(values, ",")+" FROM "+name+" WHERE nw_site = '' ON CONFLICT DO NOTHING", site); err != nil {
			return fmt.Errorf("pgdb: 复制单例模板行失败 %s：%w", table, err)
		}
	}
	o.provisioned.Store(site, true)
	return nil
}

// loadTables 启动时从已有表结构里登记单例表与租户表（建表语句不再执行时靠它）。
func loadTables(ctx context.Context, config *pgx.ConnConfig, module, schema string) {
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	for registry, query := range map[*sync.Map]string{
		&singletons:   `SELECT table_name FROM information_schema.columns WHERE table_schema = $1 AND column_name = 'nw_site'`,
		&tenantTables: `SELECT tablename FROM pg_policies WHERE schemaname = $1 AND policyname = 'nw_tenant'`,
	} {
		rows, err := conn.Query(ctx, query, schema)
		if err != nil {
			return
		}
		for rows.Next() {
			var table string
			if rows.Scan(&table) == nil {
				registry.Store(module+"."+strings.ToLower(table), true)
			}
		}
		rows.Close()
	}
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	statements, err := translate(c.owner.module, query)
	if err != nil {
		return nil, err
	}
	if len(statements) != 1 {
		return nil, errors.New("pgdb: 预编译语句只能是单条")
	}
	stmt, err := c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, statements[0])
	if err != nil {
		return nil, mapError(err)
	}
	oids, err := c.paramTypes(ctx, statements[0])
	if err != nil {
		_ = stmt.Close()
		return nil, mapError(err)
	}
	return &bytesStmt{Stmt: stmt, oids: oids, c: c, statement: statements[0]}, nil
}

// paramTypes 让服务端描述语句，取得各参数的类型（每条连接上每个语句文本只描述一次，并作为命名预编译语句复用）。
func (c *conn) paramTypes(ctx context.Context, statement string) ([]uint32, error) {
	if oids, ok := c.described[statement]; ok {
		return oids, nil
	}
	pgxConn := c.Conn.(*stdlib.Conn).Conn()
	description, err := pgxConn.Prepare(ctx, statement, statement)
	if err != nil {
		return nil, err
	}
	c.described[statement] = description.ParamOIDs
	return description.ParamOIDs, nil
}

// afterDDL 结构变更后丢弃本连接缓存的预编译语句，避免命中旧结构。
func (c *conn) afterDDL(ctx context.Context, statement string) {
	upper := strings.ToUpper(strings.TrimSpace(statement))
	if strings.HasPrefix(upper, "CREATE") || strings.HasPrefix(upper, "ALTER") || strings.HasPrefix(upper, "DROP") {
		_ = c.Conn.(*stdlib.Conn).Conn().DeallocateAll(ctx)
		c.described = map[string][]uint32{}
	}
}

// bytesRows 让名为 *_json 的文本列以字节返回：这些列在 SQLite 里是 BLOB，代码按 []byte / json.RawMessage 扫描。
type bytesRows struct {
	driver.Rows
	cols []int
	init bool
}

func (r *bytesRows) Next(dest []driver.Value) error {
	if !r.init {
		r.init = true
		for i, name := range r.Rows.Columns() {
			if name == "json" || strings.HasSuffix(name, "_json") {
				r.cols = append(r.cols, i)
			}
		}
	}
	if err := r.Rows.Next(dest); err != nil {
		return err
	}
	for _, i := range r.cols {
		if text, ok := dest[i].(string); ok {
			dest[i] = []byte(text)
		}
	}
	return nil
}

type bytesStmt struct {
	driver.Stmt
	oids      []uint32
	c         *conn
	statement string
}

func (s *bytesStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := s.c.useSite(ctx, s.statement); err != nil {
		return nil, mapError(err)
	}
	coerceArgs(s.oids, args)
	rows, err := s.Stmt.(driver.StmtQueryContext).QueryContext(ctx, args)
	if err != nil {
		return nil, mapError(err)
	}
	return &bytesRows{Rows: rows}, nil
}

func (s *bytesStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := s.c.useSite(ctx, s.statement); err != nil {
		return nil, mapError(err)
	}
	coerceArgs(s.oids, args)
	result, err := s.Stmt.(driver.StmtExecContext).ExecContext(ctx, args)
	return result, mapError(err)
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *conn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.useSite(ctx, "BEGIN"); err != nil {
		return nil, mapError(err)
	}
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	if !opts.ReadOnly {
		if _, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", []driver.NamedValue{{Ordinal: 1, Value: c.owner.lockKey}}); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	c.inTx = true
	return &txWrapper{Tx: tx, c: c}, nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	statements, err := translate(c.owner.module, query)
	if err != nil {
		debugf(c.owner.module, err, query)
		return nil, err
	}
	if err := c.useSite(ctx, query); err != nil {
		return nil, mapError(err)
	}
	execer := c.Conn.(driver.ExecerContext)
	var result driver.Result
	for i, statement := range statements {
		var bound []driver.NamedValue
		if i == 0 && len(statements) == 1 {
			bound = args
			if len(bound) > 0 {
				oids, perr := c.paramTypes(ctx, statement)
				if perr != nil {
					debugf(c.owner.module, perr, statement)
					return nil, mapError(perr)
				}
				coerceArgs(oids, bound)
			}
		}
		// SQLite 里单条语句失败不会让事务作废，代码常靠“先插入、遇唯一冲突当重复”来判断；
		// PostgreSQL 会中止整个事务，所以事务内的普通 INSERT 用保存点隔离。
		guard := c.inTx && isPlainInsert(statement)
		if guard {
			if _, err = execer.ExecContext(ctx, "SAVEPOINT nw_sp", nil); err != nil {
				return nil, mapError(err)
			}
		}
		result, err = execer.ExecContext(ctx, statement, bound)
		if guard {
			if err != nil {
				_, _ = execer.ExecContext(ctx, "ROLLBACK TO SAVEPOINT nw_sp", nil)
			}
			_, _ = execer.ExecContext(ctx, "RELEASE SAVEPOINT nw_sp", nil)
		}
		if err != nil {
			debugf(c.owner.module, err, statement)
			return nil, mapError(err)
		}
		c.afterDDL(ctx, statement)
	}
	return result, nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	statements, err := translate(c.owner.module, query)
	if err != nil {
		debugf(c.owner.module, err, query)
		return nil, err
	}
	if len(statements) != 1 {
		return nil, errors.New("pgdb: 查询只能是单条语句")
	}
	if err := c.useSite(ctx, query); err != nil {
		return nil, mapError(err)
	}
	if len(args) > 0 {
		oids, perr := c.paramTypes(ctx, statements[0])
		if perr != nil {
			debugf(c.owner.module, perr, statements[0])
			return nil, mapError(perr)
		}
		coerceArgs(oids, args)
	}
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, statements[0], args)
	if err != nil {
		debugf(c.owner.module, err, statements[0])
		return nil, mapError(err)
	}
	return &bytesRows{Rows: rows}, nil
}

// coerceArgs 按服务端推断的参数类型做宽松转换，模拟 SQLite 的动态类型：
// 整数/布尔→文本，字符串→整数/布尔/浮点，字符串→字节（避免 bytea 转义格式误解反斜杠）。
func coerceArgs(oids []uint32, args []driver.NamedValue) {
	for i := range args {
		if i >= len(oids) {
			return
		}
		args[i].Value = coerce(oids[i], args[i].Value)
	}
}

const (
	oidBool    = 16
	oidBytea   = 17
	oidName    = 19
	oidInt8    = 20
	oidInt2    = 21
	oidInt4    = 23
	oidText    = 25
	oidFloat4  = 700
	oidFloat8  = 701
	oidBpchar  = 1042
	oidVarchar = 1043
)

func coerce(oid uint32, v driver.Value) driver.Value {
	switch oid {
	case oidText, oidVarchar, oidBpchar, oidName:
		switch x := v.(type) {
		case int64:
			return strconv.FormatInt(x, 10)
		case float64:
			return strconv.FormatFloat(x, 'g', -1, 64)
		case bool:
			if x {
				return "1"
			}
			return "0"
		}
	case oidInt8, oidInt4, oidInt2:
		switch x := v.(type) {
		case string:
			if n, err := strconv.ParseInt(x, 10, 64); err == nil {
				return n
			}
		case []byte:
			if n, err := strconv.ParseInt(string(x), 10, 64); err == nil {
				return n
			}
		case bool:
			if x {
				return int64(1)
			}
			return int64(0)
		}
	case oidBool:
		switch x := v.(type) {
		case int64:
			return x != 0
		case string:
			return x == "1" || strings.EqualFold(x, "true")
		}
	case oidFloat8, oidFloat4:
		switch x := v.(type) {
		case string:
			if f, err := strconv.ParseFloat(x, 64); err == nil {
				return f
			}
		case int64:
			return float64(x)
		}
	case oidBytea:
		if x, ok := v.(string); ok {
			return []byte(x)
		}
	}
	return v
}

func isPlainInsert(statement string) bool {
	upper := strings.ToUpper(strings.TrimSpace(statement))
	return strings.HasPrefix(upper, "INSERT") && !strings.Contains(upper, "ON CONFLICT")
}

// debugf 在 NW_DEBUG 打开时把失败语句打到 stderr（不含参数值）。
func debugf(module string, err error, statement string) {
	if os.Getenv("NW_DEBUG") == "" || errors.Is(err, context.Canceled) {
		return
	}
	statement = strings.Join(strings.Fields(statement), " ")
	if len(statement) > 300 {
		statement = statement[:300] + "…"
	}
	fmt.Fprintf(os.Stderr, "pgdb[%s] %v | %s\n", module, err, statement)
}

func (c *conn) CheckNamedValue(value *driver.NamedValue) error {
	// 与 database/sql 默认的参数规整一致（int→int64、命名字节类型→[]byte、Valuer 取值），再补 PostgreSQL 需要的两项。
	converted, err := driver.DefaultParameterConverter.ConvertValue(value.Value)
	if err != nil {
		return err
	}
	switch v := converted.(type) {
	case bool:
		if v {
			converted = int64(1)
		} else {
			converted = int64(0)
		}
	case time.Time:
		converted = v.UTC().Format(time.RFC3339Nano)
	}
	value.Value = converted
	return nil
}

func (c *conn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *conn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *conn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

// Open 打开某个组件的库；等价于 sql.Open("sqlite", legacyDSN)。
func Open(legacyDSN string) (*sql.DB, error) { return sql.Open("sqlite", legacyDSN) }
