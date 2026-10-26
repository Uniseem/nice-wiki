package pgdb

import "strings"

type tokKind int

const (
	tkSpace tokKind = iota
	tkComment
	tkWord
	tkNumber
	tkString
	tkQuoted
	tkParam
	tkPunct
)

type token struct {
	kind tokKind
	text string
}

func (t token) is(word string) bool { return t.kind == tkWord && strings.EqualFold(t.text, word) }

func isWordStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isWordPart(c byte) bool { return isWordStart(c) || (c >= '0' && c <= '9') || c == '$' }

// tokenize 把 SQL 切成记号：字符串、引用标识符、注释原样保留，翻译只作用于其余部分。
func tokenize(sql string) []token {
	var out []token
	i, n := 0, len(sql)
	for i < n {
		c := sql[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			j := i
			for j < n && (sql[j] == ' ' || sql[j] == '\t' || sql[j] == '\n' || sql[j] == '\r') {
				j++
			}
			out = append(out, token{tkSpace, sql[i:j]})
			i = j
		case c == '-' && i+1 < n && sql[i+1] == '-':
			j := i
			for j < n && sql[j] != '\n' {
				j++
			}
			out = append(out, token{tkComment, sql[i:j]})
			i = j
		case c == '/' && i+1 < n && sql[i+1] == '*':
			j := strings.Index(sql[i+2:], "*/")
			if j < 0 {
				j = n
			} else {
				j = i + 2 + j + 2
			}
			out = append(out, token{tkComment, sql[i:j]})
			i = j
		case c == '\'':
			j := i + 1
			for j < n {
				if sql[j] == '\'' {
					if j+1 < n && sql[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			out = append(out, token{tkString, sql[i:j]})
			i = j
		case c == '"' || c == '`':
			j := i + 1
			for j < n && sql[j] != c {
				j++
			}
			if j < n {
				j++
			}
			text := sql[i:j]
			if c == '`' {
				text = `"` + text[1:len(text)-1] + `"`
			}
			out = append(out, token{tkQuoted, text})
			i = j
		case c == '?':
			out = append(out, token{tkParam, "?"})
			i++
		case c >= '0' && c <= '9':
			j := i
			for j < n && (sql[j] >= '0' && sql[j] <= '9' || sql[j] == '.') {
				j++
			}
			out = append(out, token{tkNumber, sql[i:j]})
			i = j
		case isWordStart(c):
			j := i
			for j < n && isWordPart(sql[j]) {
				j++
			}
			out = append(out, token{tkWord, sql[i:j]})
			i = j
		default:
			if i+2 < n && sql[i:i+3] == "->>" {
				out = append(out, token{tkPunct, "->>"})
				i += 3
				continue
			}
			if i+1 < n {
				two := sql[i : i+2]
				switch two {
				case "||", "<=", ">=", "<>", "!=", "==", "->", "::":
					out = append(out, token{tkPunct, two})
					i += 2
					continue
				}
			}
			out = append(out, token{tkPunct, string(c)})
			i++
		}
	}
	return out
}

func render(tokens []token) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(t.text)
	}
	return b.String()
}

// nextSig 返回 i 之后第一个非空白、非注释记号的下标，没有则返回 len。
func nextSig(tokens []token, i int) int {
	for i++; i < len(tokens); i++ {
		if tokens[i].kind != tkSpace && tokens[i].kind != tkComment {
			return i
		}
	}
	return len(tokens)
}

// prevSig 返回 i 之前第一个有效记号的下标，没有则返回 -1。
func prevSig(tokens []token, i int) int {
	for i--; i >= 0; i-- {
		if tokens[i].kind != tkSpace && tokens[i].kind != tkComment {
			return i
		}
	}
	return -1
}

// matchParen 返回与 tokens[open]（必须是"("）配对的")"下标，找不到返回 -1。
func matchParen(tokens []token, open int) int {
	depth := 0
	for i := open; i < len(tokens); i++ {
		if tokens[i].kind != tkPunct {
			continue
		}
		switch tokens[i].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitArgs 把 open..close 之间按顶层逗号切成参数记号段。
func splitArgs(tokens []token, open, close int) [][]token {
	var args [][]token
	depth := 0
	start := open + 1
	for i := open + 1; i < close; i++ {
		if tokens[i].kind != tkPunct {
			continue
		}
		switch tokens[i].text {
		case "(":
			depth++
		case ")":
			depth--
		case ",":
			if depth == 0 {
				args = append(args, tokens[start:i])
				start = i + 1
			}
		}
	}
	if start < close || len(args) > 0 {
		args = append(args, tokens[start:close])
	}
	return args
}

func trimSpace(tokens []token) []token {
	for len(tokens) > 0 && (tokens[0].kind == tkSpace || tokens[0].kind == tkComment) {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && (tokens[len(tokens)-1].kind == tkSpace || tokens[len(tokens)-1].kind == tkComment) {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

func words(text string) []token { return tokenize(text) }
