package pgdb

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// translateTrigger 把 SQLite 触发器翻成 PostgreSQL 的“触发器函数 + 触发器”两条语句。
// WHEN 条件一律挪进函数体（PostgreSQL 的 WHEN 子句不允许子查询）；
// SELECT RAISE(ABORT,'msg') [WHERE 条件] 翻成条件抛错。
func translateTrigger(tokens []token) ([]string, error) {
	skip := func(i int) int {
		for i < len(tokens) && (tokens[i].kind == tkSpace || tokens[i].kind == tkComment) {
			i++
		}
		return i
	}
	i := skip(0)
	expect := func(word string) error {
		i = skip(i)
		if i >= len(tokens) || !tokens[i].is(word) {
			return fmt.Errorf("pgdb: 触发器语法不支持，缺少 %s", word)
		}
		i++
		return nil
	}
	if err := expect("CREATE"); err != nil {
		return nil, err
	}
	i = skip(i)
	if i < len(tokens) && (tokens[i].is("TEMP") || tokens[i].is("TEMPORARY")) {
		i++
	}
	if err := expect("TRIGGER"); err != nil {
		return nil, err
	}
	i = skip(i)
	if i < len(tokens) && tokens[i].is("IF") {
		i = skip(i + 1)
		i = skip(i + 1) // NOT
		i = skip(i + 1) // EXISTS
	}
	nameToken := tokens[i]
	i++
	i = skip(i)
	timing := strings.ToUpper(tokens[i].text)
	if timing == "INSTEAD" {
		return nil, fmt.Errorf("pgdb: 不支持 INSTEAD OF 触发器")
	}
	if timing != "BEFORE" && timing != "AFTER" {
		timing = "AFTER"
	} else {
		i++
	}
	i = skip(i)
	event := strings.ToUpper(tokens[i].text)
	i++
	eventSQL := event
	i = skip(i)
	if event == "UPDATE" && i < len(tokens) && tokens[i].is("OF") {
		var columns []string
		i = skip(i + 1)
		for i < len(tokens) && !tokens[i].is("ON") {
			if tokens[i].kind != tkSpace {
				columns = append(columns, tokens[i].text)
			}
			i++
		}
		eventSQL = "UPDATE OF " + strings.Join(columns, " ")
		i = skip(i)
	}
	if err := expect("ON"); err != nil {
		return nil, err
	}
	i = skip(i)
	table := tokens[i].text
	i++
	i = skip(i)
	if i < len(tokens) && tokens[i].is("FOR") {
		i = skip(i + 1)
		i = skip(i + 1)
		i = skip(i + 1)
	}
	var when []token
	if i < len(tokens) && tokens[i].is("WHEN") {
		i++
		start := i
		for i < len(tokens) && !tokens[i].is("BEGIN") {
			i++
		}
		when = trimSpace(tokens[start:i])
	}
	if err := expect("BEGIN"); err != nil {
		return nil, err
	}
	bodyEnd := len(tokens) - 1
	for bodyEnd > i && !tokens[bodyEnd].is("END") {
		bodyEnd--
	}
	var body []string
	for _, statement := range splitBody(tokens[i:bodyEnd]) {
		statement = trimSpace(statement)
		if len(statement) == 0 {
			continue
		}
		translated, err := triggerStatement(statement)
		if err != nil {
			return nil, err
		}
		body = append(body, translated)
	}
	returnValue := "NULL"
	if timing == "BEFORE" {
		returnValue = "NEW"
		if event == "DELETE" {
			returnValue = "OLD"
		}
	}
	name := triggerFunctionName(nameToken.text)
	var function strings.Builder
	function.WriteString("CREATE OR REPLACE FUNCTION " + name + "() RETURNS trigger LANGUAGE plpgsql AS $nw$ BEGIN ")
	if len(when) > 0 {
		function.WriteString("IF NOT COALESCE((" + finish(rewrite(when, false)) + "), false) THEN RETURN " + returnValue + "; END IF; ")
	}
	function.WriteString(strings.Join(body, " "))
	function.WriteString(" RETURN " + returnValue + "; END $nw$")
	trigger := "CREATE OR REPLACE TRIGGER " + nameToken.text + " " + timing + " " + eventSQL + " ON " + table + " FOR EACH ROW EXECUTE FUNCTION " + name + "()"
	return []string{function.String(), trigger}, nil
}

func triggerFunctionName(trigger string) string {
	clean := strings.Trim(trigger, `"`)
	sanitized := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, clean)
	hash := fnv.New32a()
	hash.Write([]byte(clean))
	return fmt.Sprintf("nw_trg_%s_%08x", strings.ToLower(sanitized[:min(len(sanitized), 40)]), hash.Sum32())
}

// splitBody 按分号切触发器体（体内没有嵌套 BEGIN）。
func splitBody(tokens []token) [][]token {
	var out [][]token
	start, depth := 0, 0
	for i, t := range tokens {
		if t.kind != tkPunct {
			continue
		}
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
		case ";":
			if depth == 0 {
				out = append(out, tokens[start:i])
				start = i + 1
			}
		}
	}
	if start < len(tokens) {
		out = append(out, tokens[start:])
	}
	return out
}

func triggerStatement(statement []token) (string, error) {
	if firstWord(statement) == "SELECT" {
		raise := -1
		if first := nextSig(statement, indexOfWord(statement, "SELECT")); first < len(statement) && statement[first].is("RAISE") {
			raise = first
		}
		if raise >= 0 {
			open := nextSig(statement, raise)
			end := matchParen(statement, open)
			if open >= len(statement) || end < 0 {
				return "", fmt.Errorf("pgdb: RAISE 语法不支持")
			}
			args := splitArgs(statement, open, end)
			if len(args) < 2 {
				return "", fmt.Errorf("pgdb: RAISE 只支持 ABORT/FAIL 加消息")
			}
			message := strings.TrimSpace(render(args[1]))
			rest := trimSpace(statement[end+1:])
			condition := ""
			switch {
			case len(rest) == 0:
			case rest[0].is("WHERE"):
				condition = finish(rewrite(trimSpace(rest[1:]), false))
			case rest[0].is("FROM"):
				condition = "EXISTS (SELECT 1 " + finish(rewrite(rest, false)) + ")"
			default:
				return "", fmt.Errorf("pgdb: RAISE 后的子句不支持")
			}
			if condition == "" {
				return "RAISE EXCEPTION '%', " + message + ";", nil
			}
			return "IF " + condition + " THEN RAISE EXCEPTION '%', " + message + "; END IF;", nil
		}
		return "PERFORM " + finish(rewrite(trimSpace(statement[1:]), false)) + ";", nil
	}
	return finish(rewriteStatement(statement)) + ";", nil
}
