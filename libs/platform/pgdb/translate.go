package pgdb

import (
	"fmt"
	"strconv"
	"strings"
)

// TranslateScript 把 SQLite 方言的 SQL（可含多条语句和触发器）改写成 PostgreSQL 语句序列。
// 占位符 ? 改为 $1..$n；只在单条带参数语句上有意义，多语句脚本按无参数执行。
func TranslateScript(sql string) (out []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("pgdb: 语句无法翻译：%v", r)
		}
	}()
	for _, statement := range splitStatements(tokenize(sql)) {
		statement = trimSpace(statement)
		if len(statement) == 0 {
			continue
		}
		translated, err := translateStatement(statement)
		if err != nil {
			return nil, err
		}
		out = append(out, translated...)
	}
	return out, nil
}

// splitStatements 按顶层分号切分；触发器的 BEGIN…END 体内的分号不切。
func splitStatements(tokens []token) [][]token {
	var out [][]token
	start := 0
	trigger, inBody, caseDepth := false, false, 0
	first := true
	var seenWords []string
	for i, t := range tokens {
		if t.kind == tkWord {
			upper := strings.ToUpper(t.text)
			if first {
				first = false
				seenWords = seenWords[:0]
			}
			if len(seenWords) < 4 {
				seenWords = append(seenWords, upper)
			}
			if !trigger && len(seenWords) >= 2 && seenWords[0] == "CREATE" {
				for _, w := range seenWords[1:] {
					if w == "TRIGGER" {
						trigger = true
					}
				}
			}
			switch {
			case trigger && !inBody && upper == "BEGIN":
				inBody = true
			case inBody && upper == "CASE":
				caseDepth++
			case inBody && upper == "END":
				if caseDepth > 0 {
					caseDepth--
				} else {
					inBody = false
				}
			}
		}
		if t.kind == tkPunct && t.text == ";" && !inBody {
			out = append(out, tokens[start:i])
			start = i + 1
			trigger, inBody, caseDepth, first = false, false, 0, true
		}
	}
	if start < len(tokens) {
		out = append(out, tokens[start:])
	}
	return out
}

func firstWord(tokens []token) string {
	for _, t := range tokens {
		if t.kind == tkWord {
			return strings.ToUpper(t.text)
		}
	}
	return ""
}

func translateStatement(tokens []token) ([]string, error) {
	switch firstWord(tokens) {
	case "PRAGMA":
		return translatePragma(tokens)
	case "VACUUM":
		if len(tokens) > 2 {
			return nil, fmt.Errorf("pgdb: PostgreSQL 不支持 VACUUM INTO")
		}
		return []string{"SELECT 1"}, nil
	case "CREATE":
		for _, t := range tokens[:min(len(tokens), 8)] {
			if t.is("TRIGGER") {
				return translateTrigger(tokens)
			}
		}
	}
	return []string{finish(rewriteStatement(tokens))}, nil
}

// rewriteStatement 对一条完整语句做改写：INSERT OR IGNORE、DDL 类型映射，再套通用记号改写。
func rewriteStatement(tokens []token) []token {
	first := firstWord(tokens)
	ddl := first == "CREATE" || first == "ALTER"
	ignore := false
	if first == "INSERT" {
		// INSERT OR IGNORE INTO → INSERT INTO … ON CONFLICT DO NOTHING
		i := nextSig(tokens, indexOfWord(tokens, "INSERT"))
		if i < len(tokens) && tokens[i].is("OR") {
			j := nextSig(tokens, i)
			if j < len(tokens) && tokens[j].is("IGNORE") {
				ignore = true
				tokens = append(append([]token{}, tokens[:i]...), tokens[j+1:]...)
			}
		}
	}
	out := rewrite(tokens, ddl)
	if first == "INSERT" {
		out = qualifyUpsert(out)
	}
	if ignore && !hasTopLevelWord(out, "CONFLICT") {
		at := len(out)
		depth := 0
		for i, t := range out {
			if t.kind == tkPunct && t.text == "(" {
				depth++
			} else if t.kind == tkPunct && t.text == ")" {
				depth--
			} else if depth == 0 && t.is("RETURNING") {
				at = i
				break
			}
		}
		tail := append([]token{{tkSpace, " "}, {tkWord, "ON"}, {tkSpace, " "}, {tkWord, "CONFLICT"}, {tkSpace, " "}, {tkWord, "DO"}, {tkSpace, " "}, {tkWord, "NOTHING"}, {tkSpace, " "}}, out[at:]...)
		out = append(append([]token{}, out[:at]...), tail...)
	}
	return out
}

func indexOfWord(tokens []token, word string) int {
	for i, t := range tokens {
		if t.is(word) {
			return i
		}
	}
	return -1
}

func hasTopLevelWord(tokens []token, word string) bool {
	depth := 0
	for _, t := range tokens {
		if t.kind == tkPunct && t.text == "(" {
			depth++
		} else if t.kind == tkPunct && t.text == ")" {
			depth--
		} else if depth == 0 && t.is(word) {
			return true
		}
	}
	return false
}

// finish 渲染并把占位符改成 $n。
func finish(tokens []token) string {
	n := 0
	for i, t := range tokens {
		if t.kind == tkParam {
			n++
			tokens[i] = token{tkParam, "$" + strconv.Itoa(n)}
		}
	}
	return strings.TrimSpace(render(tokens))
}

var ddlTypes = map[string]string{
	"INTEGER": "BIGINT", "INT": "BIGINT", "REAL": "DOUBLE PRECISION", "BLOB": "BYTEA", "DATETIME": "TEXT", "BOOLEAN": "BIGINT", "TINYINT": "BIGINT",
}

var castTypes = map[string]string{"INTEGER": "BIGINT", "INT": "BIGINT", "REAL": "DOUBLE PRECISION", "BLOB": "BYTEA"}

func lit(text string) token { return token{tkWord, text} }

func joinTokens(parts ...any) []token {
	var out []token
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			out = append(out, tokenize(v)...)
		case []token:
			out = append(out, v...)
		case token:
			out = append(out, v)
		}
	}
	return out
}

// rewrite 是通用记号改写：函数、类型、IS、rowid、十六进制字面量等。
func rewrite(tokens []token, ddl bool) []token {
	var out []token
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.kind {
		case tkPunct:
			if t.text == "==" {
				out = append(out, token{tkPunct, "="})
				continue
			}
		case tkWord:
			lower := strings.ToLower(t.text)
			if lower == "x" && i+1 < len(tokens) && tokens[i+1].kind == tkString {
				hex := strings.Trim(tokens[i+1].text, "'")
				out = append(out, token{tkString, "'\\x" + hex + "'::bytea"})
				i++
				continue
			}
			if lower == "rowid" {
				out = append(out, lit("ctid"))
				continue
			}
			if lower == "where" {
				// SQLite 允许整数当条件（WHERE 1）；PostgreSQL 要布尔。
				if j := nextSig(tokens, i); j < len(tokens) && tokens[j].kind == tkNumber && (tokens[j].text == "1" || tokens[j].text == "0") {
					if k := nextSig(tokens, j); k >= len(tokens) || (tokens[k].kind == tkWord && !tokens[k].is("IS")) || (tokens[k].kind == tkPunct && (tokens[k].text == ")" || tokens[k].text == ";")) {
						value := "TRUE"
						if tokens[j].text == "0" {
							value = "FALSE"
						}
						out = append(out, t, token{tkSpace, " "}, lit(value))
						i = j
						continue
					}
				}
			}
			if lower == "limit" {
				// SQLite 里负的 LIMIT 表示不限；PostgreSQL 里 LIMIT NULL 才是不限。
				if j := nextSig(tokens, i); j < len(tokens) && tokens[j].kind == tkParam {
					out = append(out, t, token{tkSpace, " "}, lit("NULLIF(GREATEST("), tokens[j], token{tkPunct, ","}, lit(" -1::bigint),"), lit(" -1::bigint)"))
					i = j
					continue
				}
			}
			if lower == "sqlite_master" {
				out = append(out, joinTokens("(SELECT table_name AS name, 'table' AS type FROM information_schema.tables WHERE table_schema = current_schema() AND table_name <> 'nw_pragma') sqlite_master")...)
				continue
			}
			if lower == "glob" {
				if j := nextSig(tokens, i); j < len(tokens) && tokens[j].kind == tkString {
					op := "~"
					if k := lastSig(out); k >= 0 && out[k].is("NOT") {
						out = out[:k]
						op = "!~"
					}
					out = append(out, joinTokens(" ", op, " ", token{tkString, globToRegex(tokens[j].text)})...)
					i = j
					continue
				}
			}
			if lower == "is" {
				out = append(out, rewriteIs(tokens, &i)...)
				continue
			}
			if ddl {
				if lower == "integer" {
					if j := nextSig(tokens, i); j < len(tokens) && tokens[j].is("PRIMARY") {
						if k := nextSig(tokens, j); k < len(tokens) && tokens[k].is("KEY") {
							end := k
							if m := nextSig(tokens, k); m < len(tokens) && tokens[m].is("AUTOINCREMENT") {
								end = m
							}
							out = append(out, joinTokens("BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY")...)
							i = end
							continue
						}
					}
				}
				if mapped, ok := ddlTypes[strings.ToUpper(t.text)]; ok {
					out = append(out, lit(mapped))
					continue
				}
			}
			if j := nextSig(tokens, i); j < len(tokens) && tokens[j].kind == tkPunct && tokens[j].text == "(" {
				if end := matchParen(tokens, j); end > 0 {
					if replaced, ok := rewriteCall(lower, splitArgs(tokens, j, end), tokens, i, end); ok {
						out = append(out, replaced.tokens...)
						i = end
						if replaced.skipTo > end {
							i = replaced.skipTo
						}
						continue
					}
				}
			}
		}
		out = append(out, t)
	}
	return out
}

func rewriteIs(tokens []token, i *int) []token {
	j := nextSig(tokens, *i)
	keep := func(t token) bool {
		return t.is("NULL") || t.is("TRUE") || t.is("FALSE") || t.is("DISTINCT") || t.is("UNKNOWN")
	}
	if j < len(tokens) && tokens[j].is("NOT") {
		k := nextSig(tokens, j)
		if k < len(tokens) && !keep(tokens[k]) {
			*i = j
			return joinTokens("IS DISTINCT FROM")
		}
		return []token{tokens[*i]}
	}
	if j < len(tokens) && !keep(tokens[j]) {
		return joinTokens("IS NOT DISTINCT FROM")
	}
	return []token{tokens[*i]}
}

type replacement struct {
	tokens []token
	skipTo int
}

func argText(arg []token) string { return strings.TrimSpace(render(arg)) }

func isStringLit(arg []token) (string, bool) {
	arg = trimSpace(arg)
	if len(arg) == 1 && arg[0].kind == tkString {
		return arg[0].text[1 : len(arg[0].text)-1], true
	}
	return "", false
}

// negativeInt 识别形如 -3 的整数字面量，返回去掉负号后的数字文本。
func negativeInt(arg []token) (string, bool) {
	arg = trimSpace(arg)
	if len(arg) == 2 && arg[0].kind == tkPunct && arg[0].text == "-" && arg[1].kind == tkNumber {
		return arg[1].text, true
	}
	if len(arg) == 1 && arg[0].kind == tkNumber && strings.HasPrefix(arg[0].text, "-") {
		return arg[0].text[1:], true
	}
	return "", false
}

// jsonPath 把 SQLite 的 '$.a.b[0]' 路径转成 PostgreSQL 的 '{a,b,0}' 文本数组字面量。
func jsonPath(path string) string {
	path = strings.TrimPrefix(path, "$")
	var parts []string
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			continue
		}
		for {
			open := strings.Index(segment, "[")
			if open < 0 {
				break
			}
			if open > 0 {
				parts = append(parts, segment[:open])
			}
			closing := strings.Index(segment, "]")
			if closing < open {
				break
			}
			parts = append(parts, segment[open+1:closing])
			segment = segment[closing+1:]
		}
		if segment != "" {
			parts = append(parts, segment)
		}
	}
	return "'{" + strings.Join(parts, ",") + "}'"
}

// typedJSONExtract 处理直接作为 json_object 值的 json_extract(x, '$.a')：SQLite 里数字取出来还是数字，
// PostgreSQL 的 #>> 会变成文本，写进 JSON 后读取方按整数解码就失败；这里改用 #> 保留 JSON 原类型。
func typedJSONExtract(raw []token) ([]token, bool) {
	raw = trimSpace(raw)
	if len(raw) < 4 || !raw[0].is("json_extract") {
		return nil, false
	}
	open := nextSig(raw, 0)
	if open >= len(raw) || raw[open].kind != tkPunct || raw[open].text != "(" || matchParen(raw, open) != len(raw)-1 {
		return nil, false
	}
	parts := splitArgs(raw, open, len(raw)-1)
	if len(parts) != 2 {
		return nil, false
	}
	path, ok := isStringLit(parts[1])
	if !ok {
		return nil, false
	}
	return joinTokens("(public.nw_jsonb(", rewrite(trimSpace(parts[0]), false), ") #> ", jsonPath(path), ")"), true
}

func rewriteArgs(args [][]token) [][]token {
	out := make([][]token, len(args))
	for i, a := range args {
		out[i] = rewrite(trimSpace(a), false)
	}
	return out
}

// rewriteCall 处理需要改写的函数调用；不认识的函数原样保留。
func rewriteCall(name string, rawArgs [][]token, tokens []token, at, end int) (replacement, bool) {
	args := rewriteArgs(rawArgs)
	switch name {
	case "raise":
		// 表达式位置的 RAISE(ABORT,'msg')：条件为真时由函数抛错。
		if len(args) >= 2 {
			return replacement{tokens: joinTokens("public.nw_raise(", args[1], ")")}, true
		}
	case "json_extract":
		if len(rawArgs) == 2 {
			if path, ok := isStringLit(rawArgs[1]); ok {
				return replacement{tokens: joinTokens("(public.nw_jsonb(", args[0], ") #>> ", jsonPath(path), ")")}, true
			}
		}
	case "substr", "substring":
		// SQLite 的 substr(x, -N) 取末尾 N 个字符；PostgreSQL 里负起点会从头取，得换成 right()。
		if len(rawArgs) == 2 {
			if n, ok := negativeInt(rawArgs[1]); ok {
				return replacement{tokens: joinTokens("right(", args[0], ", ", n, ")")}, true
			}
		}
	case "json_valid":
		if len(args) == 1 {
			return replacement{tokens: joinTokens("pg_input_is_valid(public.nw_text(", args[0], "),'jsonb')")}, true
		}
	case "json_set":
		if len(rawArgs) == 3 {
			if path, ok := isStringLit(rawArgs[1]); ok {
				value := joinTokens("to_jsonb((", args[2], ")::text)")
				if v := trimSpace(args[2]); len(v) == 1 && v[0].kind == tkNumber {
					value = joinTokens("to_jsonb(", v[0].text, ")")
				}
				return replacement{tokens: joinTokens("public.nw_json_set(", args[0], ", ", jsonPath(path), ", ", value, ")")}, true
			}
		}
	case "json_remove":
		if len(rawArgs) >= 2 {
			expr := args[0]
			for _, raw := range rawArgs[1:] {
				path, ok := isStringLit(raw)
				if !ok {
					return replacement{}, false
				}
				expr = joinTokens("public.nw_json_remove(", expr, ", ", jsonPath(path), ")")
			}
			return replacement{tokens: expr}, true
		}
	case "json_type":
		if len(rawArgs) == 2 {
			if path, ok := isStringLit(rawArgs[1]); ok {
				return replacement{tokens: joinTokens("public.nw_json_type(public.nw_jsonb(", args[0], ") #> ", jsonPath(path), ")")}, true
			}
		}
		if len(rawArgs) == 1 {
			return replacement{tokens: joinTokens("public.nw_json_type(public.nw_jsonb(", args[0], "))")}, true
		}
	case "sum":
		// SQLite 里比较结果是 0/1，SUM(a=b) 常用来计数；PostgreSQL 里要先转成整数。
		if len(args) == 1 && !startsWithWord(args[0], "CASE") && hasComparison(args[0]) {
			return replacement{tokens: joinTokens("SUM((", args[0], ")::int)")}, true
		}
	case "json_array_length":
		if len(args) == 1 {
			return replacement{tokens: joinTokens("jsonb_array_length(public.nw_jsonb(", args[0], "))")}, true
		}
	case "json_object":
		// SQLite 里 NOT EXISTS(...) 这类布尔表达式是整数 0/1，PostgreSQL 里是 JSON 布尔；读取方按整数解码，所以转成整数。
		wrapped := make([][]token, len(args))
		for i, a := range args {
			wrapped[i] = a
			if i%2 == 1 && (startsWithWord(a, "NOT") || startsWithWord(a, "EXISTS")) {
				wrapped[i] = joinTokens("(", a, ")::int")
			} else if i%2 == 1 {
				if typed, ok := typedJSONExtract(rawArgs[i]); ok {
					wrapped[i] = typed
				}
			}
		}
		return replacement{tokens: joinTokens("jsonb_build_object(", joinArgs(wrapped), ")")}, true
	case "json_array":
		return replacement{tokens: joinTokens("jsonb_build_array(", joinArgs(args), ")")}, true
	case "json_each":
		if len(rawArgs) >= 1 {
			source := joinTokens("public.nw_jsonb(", args[0], ")")
			if len(rawArgs) == 2 {
				if path, ok := isStringLit(rawArgs[1]); ok {
					source = joinTokens("(public.nw_jsonb(", args[0], ") #> ", jsonPath(path), ")")
				}
			}
			alias := "value"
			skip := end
			if j := nextSig(tokens, end); j < len(tokens) {
				if tokens[j].is("AS") {
					j = nextSig(tokens, j)
				}
				if j < len(tokens) && tokens[j].kind == tkWord && !isClauseWord(tokens[j].text) {
					alias = tokens[j].text
					skip = j
				}
			}
			return replacement{tokens: joinTokens("LATERAL (SELECT public.nw_json_value(e) AS value FROM jsonb_array_elements(", source, ") AS e) AS ", alias), skipTo: skip}, true
		}
	case "julianday":
		if len(args) == 1 {
			return replacement{tokens: joinTokens("(EXTRACT(EPOCH FROM (", args[0], ")::timestamptz)/86400.0)")}, true
		}
	case "unixepoch":
		if len(args) == 1 {
			return replacement{tokens: joinTokens("EXTRACT(EPOCH FROM (", args[0], ")::timestamptz)")}, true
		}
	case "strftime":
		return rewriteStrftime(rawArgs, args)
	case "typeof":
		// CHECK(typeof(x)='integer') 这类类型断言在强类型库里恒真。
		j := nextSig(tokens, end)
		if j < len(tokens) && tokens[j].kind == tkPunct && tokens[j].text == "=" {
			k := nextSig(tokens, j)
			if k < len(tokens) && tokens[k].kind == tkString {
				return replacement{tokens: joinTokens("TRUE"), skipTo: k}, true
			}
		}
	case "randomblob":
		if len(args) == 1 {
			return replacement{tokens: joinTokens("decode(substr(md5(random()::text || clock_timestamp()::text),1,2*(", args[0], ")),'hex')")}, true
		}
	case "hex":
		if len(args) == 1 {
			return replacement{tokens: joinTokens("upper(encode(", args[0], ",'hex'))")}, true
		}
	case "max", "min":
		if len(args) >= 2 {
			fn := "GREATEST"
			if name == "min" {
				fn = "LEAST"
			}
			return replacement{tokens: joinTokens(fn, "(", joinArgs(args), ")")}, true
		}
	case "cast":
		return rewriteCast(args)
	}
	return replacement{}, false
}

func startsWithWord(tokens []token, word string) bool {
	tokens = trimSpace(tokens)
	return len(tokens) > 0 && tokens[0].is(word)
}

// hasComparison 判断记号段顶层是否含比较运算（出现在括号内的不算）。
func hasComparison(tokens []token) bool {
	depth := 0
	for _, t := range tokens {
		if t.kind == tkPunct {
			switch t.text {
			case "(":
				depth++
			case ")":
				depth--
			case "=", "<", ">", "<=", ">=", "<>", "!=":
				if depth == 0 {
					return true
				}
			}
		}
		if depth == 0 && t.kind == tkWord && (t.is("IN") || t.is("LIKE") || t.is("IS") || t.is("BETWEEN")) {
			return true
		}
	}
	return false
}

func lastSig(tokens []token) int {
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].kind != tkSpace && tokens[i].kind != tkComment {
			return i
		}
	}
	return -1
}

// globToRegex 把 SQLite GLOB 模式（* ? [..]）转成整串匹配的正则字面量。
func globToRegex(literal string) string {
	pattern := literal[1 : len(literal)-1]
	var b strings.Builder
	b.WriteString("'^")
	inClass := false
	for _, r := range pattern {
		switch {
		case inClass:
			if r == ']' {
				inClass = false
			}
			b.WriteRune(r)
		case r == '[':
			inClass = true
			b.WriteRune(r)
		case r == '*':
			b.WriteString(".*")
		case r == '?':
			b.WriteString(".")
		case strings.ContainsRune(`.()+|^${}\`, r):
			b.WriteRune('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString("$'")
	return b.String()
}

func isClauseWord(w string) bool {
	switch strings.ToUpper(w) {
	case "WHERE", "ON", "JOIN", "LEFT", "INNER", "CROSS", "GROUP", "ORDER", "LIMIT", "UNION", "HAVING", "USING", "SET", "SELECT", "FROM", "RETURNING":
		return true
	}
	return false
}

func joinArgs(args [][]token) []token {
	var out []token
	for i, a := range args {
		if i > 0 {
			out = append(out, token{tkPunct, ","})
		}
		out = append(out, a...)
	}
	return out
}

// rewriteCast 处理 CAST(x AS 类型)：INTEGER→BIGINT；BLOB/TEXT 互转按字节语义用 nw_text。
func rewriteCast(args [][]token) (replacement, bool) {
	if len(args) != 1 {
		return replacement{}, false
	}
	body := args[0]
	depth := 0
	as := -1
	for i, t := range body {
		if t.kind == tkPunct && t.text == "(" {
			depth++
		} else if t.kind == tkPunct && t.text == ")" {
			depth--
		} else if depth == 0 && t.is("AS") {
			as = i
		}
	}
	if as < 0 {
		return replacement{}, false
	}
	expr, typ := trimSpace(body[:as]), strings.ToUpper(strings.TrimSpace(render(body[as+1:])))
	switch typ {
	case "TEXT":
		return replacement{tokens: joinTokens("public.nw_text(", expr, ")")}, true
	case "BLOB":
		return replacement{tokens: joinTokens("public.nw_bytes(", expr, ")")}, true
	}
	if mapped, ok := castTypes[typ]; ok {
		return replacement{tokens: joinTokens("CAST(", expr, " AS ", mapped, ")")}, true
	}
	return replacement{}, false
}

// rewriteStrftime 只覆盖代码里实际出现的两种格式：带毫秒的 ISO 和到秒的 ISO。
func rewriteStrftime(rawArgs [][]token, args [][]token) (replacement, bool) {
	if len(args) < 2 {
		return replacement{}, false
	}
	format, ok := isStringLit(rawArgs[0])
	if !ok {
		return replacement{}, false
	}
	var picture string
	switch format {
	case "%Y-%m-%dT%H:%M:%fZ":
		picture = `YYYY-MM-DD"T"HH24:MI:SS.MS"Z"`
	case "%Y-%m-%dT%H:%M:%S":
		picture = `YYYY-MM-DD"T"HH24:MI:SS`
	default:
		return replacement{}, false
	}
	value := joinTokens("(", args[1], ")::timestamptz")
	unix := false
	var shifts [][]token
	for _, m := range rawArgs[2:] {
		if text, ok := isStringLit(m); ok && strings.EqualFold(text, "unixepoch") {
			unix = true
			continue
		}
		shifts = append(shifts, rewrite(trimSpace(m), false))
	}
	if unix {
		value = joinTokens("to_timestamp(", args[1], ")")
	}
	for _, shift := range shifts {
		if text, ok := isStringLit(shift); ok {
			value = joinTokens("(", value, " + interval '", sqliteInterval(text), "')")
		} else {
			value = joinTokens("(", value, " + (", shift, ")::interval)")
		}
	}
	return replacement{tokens: joinTokens("to_char((", value, ") AT TIME ZONE 'UTC', '", picture, "')")}, true
}

// sqliteInterval 把 '+1 days' / '-3 seconds' 这类修饰符改成 PostgreSQL 间隔。
func sqliteInterval(text string) string {
	return strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(text), "+"), "'", "")
}

func translatePragma(tokens []token) ([]string, error) {
	var name string
	var rest []token
	for i, t := range tokens {
		if t.kind == tkWord && !t.is("PRAGMA") {
			name = strings.ToLower(t.text)
			rest = tokens[i+1:]
			break
		}
	}
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(render(rest)), "="))
	switch name {
	case "user_version":
		if strings.TrimSpace(render(rest)) == "" {
			return []string{"SELECT COALESCE((SELECT value FROM nw_pragma WHERE name='user_version'),0)"}, nil
		}
		return []string{"INSERT INTO nw_pragma(name,value) VALUES('user_version'," + value + ") ON CONFLICT(name) DO UPDATE SET value=excluded.value"}, nil
	case "table_info":
		table := strings.Trim(strings.TrimSpace(strings.Trim(strings.TrimSpace(render(rest)), "()")), `"'`+"`")
		return []string{"SELECT (c.ordinal_position-1)::int AS cid, c.column_name::text AS name, c.data_type::text AS type, (CASE WHEN c.is_nullable='NO' THEN 1 ELSE 0 END) AS notnull, c.column_default::text AS dflt_value, " +
			"(SELECT COUNT(*) FROM information_schema.table_constraints t JOIN information_schema.key_column_usage k ON k.constraint_name=t.constraint_name AND k.table_schema=t.table_schema " +
			"WHERE t.constraint_type='PRIMARY KEY' AND t.table_schema=c.table_schema AND t.table_name=c.table_name AND k.column_name=c.column_name)::int AS pk " +
			"FROM information_schema.columns c WHERE c.table_schema=current_schema() AND c.table_name='" + strings.ReplaceAll(table, "'", "''") + "' ORDER BY c.ordinal_position"}, nil
	case "journal_mode":
		return []string{"SELECT 'wal'"}, nil
	case "page_size":
		return []string{"SELECT 4096"}, nil
	case "page_count":
		return []string{"SELECT (pg_database_size(current_database())/4096)::bigint"}, nil
	case "secure_delete":
		return []string{"SELECT 0"}, nil
	case "database_list":
		return []string{"SELECT 0 AS seq, 'main'::text AS name, ''::text AS file"}, nil
	case "integrity_check", "quick_check":
		return []string{"SELECT 'ok'"}, nil
	case "foreign_key_check":
		return []string{"SELECT 1 WHERE false"}, nil
	}
	return []string{"SELECT 1"}, nil
}
