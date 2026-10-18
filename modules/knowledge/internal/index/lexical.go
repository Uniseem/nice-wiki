// Package index 维护本域独立索引代数，不修改正文权威数据。
package index

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"strings"
	"unicode"
)

const TokenizerVersion = "unicode-code-cjk-bigram-v1"

// TokenizeText 固定保留代码标识符大小写，中日韩连续文字使用双字词项。
// 单字片段保留单字；标点、空白分隔词项，组合标记留在前词中。
func TokenizeText(text string) []string {
	terms := make([]string, 0)
	var word strings.Builder
	var cjk []rune
	flushWord := func() {
		if word.Len() != 0 {
			terms = append(terms, word.String())
			word.Reset()
		}
	}
	flushCJK := func() {
		if len(cjk) == 1 {
			terms = append(terms, string(cjk))
		}
		for i := 1; i < len(cjk); i++ {
			terms = append(terms, string(cjk[i-1:i+1]))
		}
		cjk = cjk[:0]
	}
	for _, r := range text {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			flushWord()
			cjk = append(cjk, r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || (unicode.IsMark(r) && word.Len() != 0) {
			flushCJK()
			word.WriteRune(r)
		} else {
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return terms
}

type BuildLexicalIndexInput struct {
	BuildID  string `json:"buildId"`
	Language string `json:"language"`
	LeaseID  string `json:"leaseId"`
}
type BuildLexicalIndexResult struct {
	Indexed          int    `json:"indexed"`
	TokenizerVersion string `json:"tokenizerVersion"`
}

// BuildLexicalIndex 在同一租约事务内保存词频与长度；空词项也有一行完整记录。
func BuildLexicalIndex(ctx context.Context, op OperationContext, input BuildLexicalIndexInput) (BuildLexicalIndexResult, error) {
	work, tx, record, cancel, err := BeginBuildWrite(ctx, op, input.BuildID, input.LeaseID)
	if err != nil {
		return BuildLexicalIndexResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	if record.Language != input.Language {
		return BuildLexicalIndexResult{}, platform.NewError(platform.CodeIntegrityError, false, "词项索引语言不匹配")
	}
	chunks, err := ReadChunks(work, tx, input.BuildID)
	if err != nil {
		return BuildLexicalIndexResult{}, err
	}
	for _, chunk := range chunks {
		frequency := make(map[string]int)
		terms := TokenizeText(chunk.Text)
		for _, term := range terms {
			frequency[term]++
		}
		raw, err := json.Marshal(frequency)
		if err != nil {
			return BuildLexicalIndexResult{}, platform.NewError(platform.CodeInternal, false, "词项无法编码")
		}
		_, err = tx.ExecContext(work, `INSERT INTO terms(build_id,chunk_id,language,tokenizer_version,terms_json,length) VALUES(?,?,?,?,?,?) ON CONFLICT(build_id,chunk_id) DO UPDATE SET language=excluded.language,tokenizer_version=excluded.tokenizer_version,terms_json=excluded.terms_json,length=excluded.length`, input.BuildID, chunk.ChunkId, input.Language, TokenizerVersion, raw, len(terms))
		if err != nil {
			return BuildLexicalIndexResult{}, storageError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BuildLexicalIndexResult{}, storageError(err)
	}
	return BuildLexicalIndexResult{Indexed: len(chunks), TokenizerVersion: TokenizerVersion}, nil
}
