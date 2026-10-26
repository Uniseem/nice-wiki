package search

import (
	"context"
	"math"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/searchclient"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type SearchLexicalInput struct {
	Normalized string `json:"normalized"`
	Language   string `json:"language"`
	Limit      int    `json:"limit"`
	// Documents 非空时只在这些文档里排名（词频统计仍按全站）。
	Documents map[string]bool `json:"-"`
}
type SearchLexicalResult struct {
	Candidates []gen.Candidate `json:"candidates"`
}

// SearchLexical 在固定 active 快照内使用 BM25，原始分数仅决定名次。
func SearchLexical(ctx context.Context, op OperationContext, input SearchLexicalInput) (SearchLexicalResult, error) {
	snapshot, err := activeSnapshot(ctx, op)
	if err != nil {
		return SearchLexicalResult{}, err
	}
	if snapshot.Language != input.Language {
		return SearchLexicalResult{}, platform.NewError(platform.CodeIntegrityError, false, "检索语言与索引基准语言不一致")
	}
	queryTerms := make(map[string]bool)
	for _, term := range indexer.TokenizeText(input.Normalized) {
		queryTerms[term] = true
	}
	if runtime, err := searchRuntime(ctx); err == nil {
		if client, store, ok := runtime.remote(); ok {
			terms := make([]string, 0, len(queryTerms))
			for term := range queryTerms {
				terms = append(terms, term)
			}
			sort.Strings(terms)
			documents := make([]string, 0, len(input.Documents))
			for id := range input.Documents {
				documents = append(documents, id)
			}
			sort.Strings(documents)
			found, err := client.Search(ctx, op.Actor.SiteID, searchclient.Query{SpaceID: snapshot.Space.SpaceId, Terms: terms, Documents: documents, Limit: min(80, max(input.Limit, 0))})
			if err != nil {
				return SearchLexicalResult{}, err
			}
			return remoteCandidates(ctx, op, store, snapshot.Space, found.Lexical, true)
		}
	}
	frequency := make(map[string]int)
	totalLength := 0
	for _, row := range snapshot.Rows {
		totalLength += row.Length
		for term := range queryTerms {
			if row.Terms[term] > 0 {
				frequency[term]++
			}
		}
	}
	type ranked struct {
		candidate gen.Candidate
		score     float64
	}
	ranks := make([]ranked, 0)
	if len(snapshot.Rows) > 0 && totalLength > 0 {
		average := float64(totalLength) / float64(len(snapshot.Rows))
		for _, row := range snapshot.Rows {
			if len(input.Documents) > 0 && !input.Documents[row.Chunk.Document.DocumentId] {
				continue
			}
			score := 0.0
			for term := range queryTerms {
				tf := float64(row.Terms[term])
				if tf == 0 {
					continue
				}
				df := float64(frequency[term])
				idf := math.Log(1 + (float64(len(snapshot.Rows))-df+0.5)/(df+0.5))
				score += idf * tf * 2.2 / (tf + 1.2*(0.25+0.75*float64(row.Length)/average))
			}
			if score > 0 {
				ranks = append(ranks, ranked{candidate: gen.Candidate{Chunk: row.Chunk}, score: score})
			}
		}
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}
		return ranks[i].candidate.Chunk.ChunkId < ranks[j].candidate.Chunk.ChunkId
	})
	result := SearchLexicalResult{Candidates: make([]gen.Candidate, 0)}
	for rank, row := range ranks[:min(len(ranks), min(80, max(input.Limit, 0)))] {
		row.candidate.LexicalRank = rank + 1
		result.Candidates = append(result.Candidates, row.candidate)
	}
	return result, nil
}

// remoteCandidates 把检索服务命中的 (构建, 分块) 还原成带正文的候选，名次沿用服务给的顺序。
func remoteCandidates(ctx context.Context, op OperationContext, store RemoteStore, space gen.EmbeddingSpace, hits []searchclient.Hit, lexical bool) (SearchLexicalResult, error) {
	chunks, err := store.ChunksOfHits(ctx, op, space, hits)
	if err != nil {
		return SearchLexicalResult{}, err
	}
	result := SearchLexicalResult{Candidates: make([]gen.Candidate, 0, len(hits))}
	rank := 0
	for _, hit := range hits {
		chunk, ok := chunks[hit.ChunkID]
		if !ok {
			continue
		}
		rank++
		candidate := gen.Candidate{Chunk: chunk}
		if lexical {
			candidate.LexicalRank = rank
		} else {
			candidate.VectorRank = rank
		}
		result.Candidates = append(result.Candidates, candidate)
	}
	return result, nil
}
