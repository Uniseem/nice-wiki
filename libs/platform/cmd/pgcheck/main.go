// pgcheck 离线检查 SQLite 方言的 SQL 在 PostgreSQL 上是否可用：
// 先把各组件的迁移用 pgdb 翻译后真正建进临时 schema，再把代码里的 SQL 字符串逐条 PREPARE。
// 用法：go run ./cmd/pgcheck --repo <仓库根> --dsn <连接串> [--module 名] [--verbose]
package main

import (
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"nice-wiki/libs/platform/pgdb"
)

type statement struct {
	file string
	line int
	sql  string
}

var sqlStart = regexp.MustCompile(`(?is)^\s*(SELECT|INSERT|UPDATE|DELETE|WITH|CREATE|ALTER|DROP|PRAGMA|REPLACE)\b`)

func main() {
	repo := flag.String("repo", ".", "仓库根")
	dsn := flag.String("dsn", "", "PostgreSQL 连接串")
	only := flag.String("module", "", "只检查某个组件")
	verbose := flag.Bool("verbose", false, "输出通过的语句数")
	flag.Parse()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "连接失败：", err)
		os.Exit(2)
	}
	defer conn.Close(ctx)
	if err := pgdb.InstallHelpers(ctx, conn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	modules, _ := filepath.Glob(filepath.Join(*repo, "modules", "*"))
	sort.Strings(modules)
	failures := 0
	platformDDL := goStatements(filepath.Join(*repo, "libs", "platform", "inbox"))
	for _, dir := range modules {
		name := filepath.Base(dir)
		if *only != "" && name != *only {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "migrations")); err != nil {
			continue
		}
		failures += checkModule(ctx, conn, dir, name, platformDDL, *verbose)
	}
	fmt.Printf("\n不兼容语句共 %d 条\n", failures)
	if failures > 0 {
		os.Exit(1)
	}
}

func schemaName(module string, control bool) string {
	s := "lint_" + strings.ReplaceAll(module, "-", "_")
	if control {
		s += "_control"
	}
	return s
}

func exec(ctx context.Context, conn *pgx.Conn, label, sql string) error {
	translated, err := pgdb.TranslateScript(sql)
	if err != nil {
		return fmt.Errorf("翻译失败：%w", err)
	}
	for _, statement := range translated {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("%w\n      语句：%s", err, abbreviate(statement, 220))
		}
	}
	return nil
}

func abbreviate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func checkModule(ctx context.Context, conn *pgx.Conn, dir, name string, platformDDL []statement, verbose bool) int {
	failures := 0
	main, control := schemaName(name, false), schemaName(name, true)
	for _, s := range []string{main, control} {
		_, _ = conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+s+" CASCADE")
		_, _ = conn.Exec(ctx, "CREATE SCHEMA "+s)
		_, _ = conn.Exec(ctx, "CREATE TABLE "+s+".nw_pragma(name text PRIMARY KEY, value bigint NOT NULL)")
	}
	apply := func(schema, path string) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		_, _ = conn.Exec(ctx, "SET search_path = "+schema+", public")
		if err := exec(ctx, conn, path, string(raw)); err != nil {
			failures++
			fmt.Printf("[%s] 迁移失败 %s\n    %v\n", name, strings.TrimPrefix(path, dir+"/"), err)
		}
	}
	_, _ = conn.Exec(ctx, "SET search_path = "+main+", public")
	for _, pass := range []string{"CREATE TABLE", "CREATE"} {
		for _, s := range platformDDL {
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s.sql)), pass) {
				_ = exec(ctx, conn, s.file, s.sql)
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "migrations", "*.sql"))
	sort.Strings(files)
	for _, f := range files {
		apply(main, f)
	}
	controlFiles, _ := filepath.Glob(filepath.Join(dir, "migrations", "control", "*.sql"))
	sort.Strings(controlFiles)
	for _, f := range controlFiles {
		apply(control, f)
	}
	_, _ = conn.Exec(ctx, "SET search_path = "+main+", "+control+", public")
	var statements []statement
	statements = goStatements(dir)
	ok := 0
	for _, s := range statements {
		upper := strings.ToUpper(strings.TrimSpace(s.sql))
		if strings.HasPrefix(upper, "CREATE") || strings.HasPrefix(upper, "ALTER") || strings.HasPrefix(upper, "DROP") {
			_ = exec(ctx, conn, s.file, s.sql)
			continue
		}
		translated, err := pgdb.TranslateScript(s.sql)
		if err != nil {
			failures++
			fmt.Printf("[%s] %s:%d 翻译失败：%v\n", name, s.file, s.line, err)
			continue
		}
		bad := false
		for _, t := range translated {
			if _, err := conn.Exec(ctx, "PREPARE nw_lint AS "+t); err != nil {
				if strings.Contains(err.Error(), "at end of input") || strings.Contains(err.Error(), "unterminated quoted identifier") {
					break // 运行时拼接的 SQL 片段，不是完整语句
				}
				failures++
				bad = true
				fmt.Printf("[%s] %s:%d %v\n      语句：%s\n", name, s.file, s.line, strings.Split(err.Error(), "\n")[0], abbreviate(t, 260))
				break
			}
			_, _ = conn.Exec(ctx, "DEALLOCATE nw_lint")
		}
		if !bad {
			ok++
		}
	}
	if verbose {
		fmt.Printf("[%s] 通过 %d 条\n", name, ok)
	}
	return failures
}

// goStatements 收集目录下 Go 文件里所有看起来是 SQL 的字符串（含常量与 + 拼接）。
func goStatements(dirs ...string) []statement {
	var out []statement
	for _, root := range dirs {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".gen.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil
			}
			consts := map[string]string{}
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.ValueSpec); ok {
					for i, name := range spec.Names {
						if i < len(spec.Values) {
							if v, ok := evalString(spec.Values[i], consts); ok {
								consts[name.Name] = v
							}
						}
					}
				}
				return true
			})
			ast.Inspect(file, func(n ast.Node) bool {
				expr, ok := n.(ast.Expr)
				if !ok {
					return true
				}
				switch expr.(type) {
				case *ast.BasicLit, *ast.BinaryExpr:
				default:
					return true
				}
				v, ok := evalString(expr, consts)
				if !ok || len(v) < 12 || !sqlStart.MatchString(v) || !strings.ContainsAny(v, " \n") {
					return true
				}
				out = append(out, statement{file: strings.TrimPrefix(path, root+"/"), line: fset.Position(expr.Pos()).Line, sql: v})
				return false
			})
			return nil
		})
	}
	return out
}

func evalString(e ast.Expr, consts map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		l, ok := evalString(v.X, consts)
		if !ok {
			return "", false
		}
		r, ok := evalString(v.Y, consts)
		return l + r, ok
	case *ast.Ident:
		s, ok := consts[v.Name]
		return s, ok
	case *ast.ParenExpr:
		return evalString(v.X, consts)
	}
	return "", false
}
