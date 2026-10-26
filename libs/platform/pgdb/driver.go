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
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
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
	`CREATE OR REPLACE FUNCTION public.nw_json_set(x text, p text[], v jsonb) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT jsonb_set(public.nw_jsonb(x), p, v)::text $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_set(x bytea, p text[], v jsonb) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT convert_to(jsonb_set(public.nw_jsonb(x), p, v)::text, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_remove(x text, p text[]) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT (public.nw_jsonb(x) #- p)::text $$`,
	`CREATE OR REPLACE FUNCTION public.nw_json_remove(x bytea, p text[]) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT convert_to((public.nw_jsonb(x) #- p)::text, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_raise(msg text) RETURNS bigint LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '%', msg; END $$`,
	`CREATE OR REPLACE FUNCTION public.nw_text(b bytea) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT convert_from(b, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_text(v anyelement) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT v::text $$`,
	`CREATE OR REPLACE FUNCTION public.nw_bytes(t text) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT convert_to(t, 'UTF8') $$`,
	`CREATE OR REPLACE FUNCTION public.nw_bytes(b bytea) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$ SELECT b $$`,
	`CREATE OR REPLACE FUNCTION public.nw_jsonb(t text) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT CASE WHEN t IS NULL OR NOT pg_input_is_valid(t, 'jsonb') THEN NULL ELSE t::jsonb END $$`,
	`CREATE OR REPLACE FUNCTION public.nw_jsonb(b bytea) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT public.nw_jsonb(convert_from(b, 'UTF8')) $$`,
	`CREATE OR REPLACE FUNCTION public.nw_jsonb(j jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT j $$`,
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

func (Driver) OpenConnector(name string) (driver.Connector, error) {
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
	config = config.Copy()
	config.RuntimeParams["search_path"] = quoteIdent(schema) + ",public"
	config.RuntimeParams["application_name"] = "nw-" + c.module
	hash := fnv.New64a()
	hash.Write([]byte(schema))
	return &connector{inner: stdlib.GetConnector(*config), lockKey: int64(hash.Sum64() >> 1), module: c.module, schema: schema}, nil
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
	return nil
}

type connector struct {
	inner   driver.Connector
	lockKey int64
	module  string
	schema  string
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	inner, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &conn{Conn: inner, owner: c}, nil
}

func (c *connector) Driver() driver.Driver { return Driver{} }

// conn 包装 pgx 的 database/sql 连接：翻译语句、规整参数、写事务取建议锁。
type conn struct {
	driver.Conn
	owner *connector
	inTx  bool
}

// txWrapper 在提交或回滚后清掉连接的“事务中”标记。
type txWrapper struct {
	driver.Tx
	c *conn
}

func (t *txWrapper) Commit() error   { t.c.inTx = false; return t.Tx.Commit() }
func (t *txWrapper) Rollback() error { t.c.inTx = false; return t.Tx.Rollback() }

var cache sync.Map // SQLite 语句文本 -> []string

func translate(query string) ([]string, error) {
	if cached, ok := cache.Load(query); ok {
		return cached.([]string), nil
	}
	translated, err := TranslateScript(query)
	if err != nil {
		return nil, err
	}
	if len(translated) == 0 {
		translated = []string{"SELECT 1"}
	}
	cache.Store(query, translated)
	return translated, nil
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	statements, err := translate(query)
	if err != nil {
		return nil, err
	}
	if len(statements) != 1 {
		return nil, errors.New("pgdb: 预编译语句只能是单条")
	}
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, statements[0])
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *conn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
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
	statements, err := translate(query)
	if err != nil {
		debugf(c.owner.module, err, query)
		return nil, err
	}
	execer := c.Conn.(driver.ExecerContext)
	var result driver.Result
	for i, statement := range statements {
		var bound []driver.NamedValue
		if i == 0 && len(statements) == 1 {
			bound = args
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
	}
	return result, nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	statements, err := translate(query)
	if err != nil {
		debugf(c.owner.module, err, query)
		return nil, err
	}
	if len(statements) != 1 {
		return nil, errors.New("pgdb: 查询只能是单条语句")
	}
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, statements[0], args)
	if err != nil {
		debugf(c.owner.module, err, statements[0])
	}
	return rows, mapError(err)
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
	if _, ok := value.Value.(driver.Valuer); ok {
		converted, err := driver.DefaultParameterConverter.ConvertValue(value.Value)
		if err != nil {
			return err
		}
		value.Value = converted
	}
	switch v := value.Value.(type) {
	case bool:
		if v {
			value.Value = int64(1)
		} else {
			value.Value = int64(0)
		}
	case time.Time:
		value.Value = v.UTC().Format(time.RFC3339Nano)
	}
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
