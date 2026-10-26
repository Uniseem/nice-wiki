package pgdb

import "strings"

var upsertKeywords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`CASE WHEN THEN ELSE END AND OR NOT NULL IS IN LIKE BETWEEN EXISTS SELECT FROM WHERE AS TRUE FALSE DISTINCT
		ASC DESC EXCLUDED CURRENT_TIMESTAMP INTERVAL INTEGER BIGINT TEXT NUMERIC DOUBLE PRECISION BYTEA BOOLEAN JSONB LIMIT OFFSET ORDER BY GROUP
		HAVING UNION ALL JOIN ON USING LEFT INNER OUTER CROSS COLLATE ESCAPE RETURNING DO UPDATE SET NOTHING CONFLICT`) {
		upsertKeywords[w] = true
	}
}

// qualifyUpsert 给 INSERT … ON CONFLICT DO UPDATE 的 SET 右侧和 WHERE 里的裸列名加上目标表限定：
// SQLite 里它们指向已有行，PostgreSQL 里会和 excluded 混淆而报“列引用不明确”。
func qualifyUpsert(tokens []token) []token {
	into := indexOfWord(tokens, "INTO")
	if into < 0 {
		return tokens
	}
	t := nextSig(tokens, into)
	if t >= len(tokens) {
		return tokens
	}
	qualifier := tokens[t].text
	if a := nextSig(tokens, t); a < len(tokens) && tokens[a].is("AS") {
		if n := nextSig(tokens, a); n < len(tokens) {
			qualifier = tokens[n].text
		}
	}
	// 找顶层的 DO UPDATE SET
	depth, set := 0, -1
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		if tk.kind == tkPunct && tk.text == "(" {
			depth++
		} else if tk.kind == tkPunct && tk.text == ")" {
			depth--
		} else if depth == 0 && tk.is("DO") {
			u := nextSig(tokens, i)
			if u < len(tokens) && tokens[u].is("UPDATE") {
				if s := nextSig(tokens, u); s < len(tokens) && tokens[s].is("SET") {
					set = s
					break
				}
			}
		}
	}
	if set < 0 {
		return tokens
	}
	out := append([]token{}, tokens[:set+1]...)
	depth, lhs, skipDepth := 0, true, -1
	for i := set + 1; i < len(tokens); i++ {
		tk := tokens[i]
		switch {
		case tk.kind == tkPunct && tk.text == "(":
			depth++
		case tk.kind == tkPunct && tk.text == ")":
			depth--
			if skipDepth >= 0 && depth < skipDepth {
				skipDepth = -1
			}
		case depth == 0 && tk.kind == tkPunct && tk.text == ",":
			lhs = true
		case depth == 0 && tk.kind == tkPunct && tk.text == "=":
			lhs = false
		case depth == 0 && tk.is("WHERE"):
			lhs = false
		case depth == 0 && tk.is("RETURNING"):
			out = append(out, tokens[i:]...)
			return out
		case tk.is("SELECT"):
			if skipDepth < 0 {
				skipDepth = depth
			}
		case tk.kind == tkWord && !lhs && skipDepth < 0 && !upsertKeywords[strings.ToUpper(tk.text)]:
			prev, next := prevSig(tokens, i), nextSig(tokens, i)
			isPrefix := prev >= 0 && tokens[prev].kind == tkPunct && (tokens[prev].text == "." || tokens[prev].text == "::")
			isCall := next < len(tokens) && tokens[next].kind == tkPunct && (tokens[next].text == "(" || tokens[next].text == ".")
			if !isPrefix && !isCall {
				out = append(out, token{tkWord, qualifier + "." + tk.text})
				continue
			}
		}
		out = append(out, tk)
	}
	return out
}
