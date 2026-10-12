package retrieve

import (
	"context"
	"errors"
)

type Planner interface {
	Next(context.Context, string, []Unit, int) (string, bool, error)
}
type Retriever struct {
	Tools     Tools
	Planner   Planner
	Rounds    int
	ToolCalls int
}

func (r Retriever) Retrieve(ctx context.Context, question string) ([]Unit, error) {
	if r.Tools == nil {
		return nil, errors.New("检索工具尚未配置")
	}
	rounds := r.Rounds
	if rounds < 1 || rounds > 3 {
		rounds = 2
	}
	bounded := NewBoundedTools(r.Tools, r.ToolCalls)
	query := question
	result := []Unit{}
	for round := 0; round < rounds; round++ {
		units, err := bounded.Search(ctx, query, 20)
		if err != nil {
			return nil, err
		}
		result = append(result, units...)
		if r.Planner == nil {
			break
		}
		next, done, err := r.Planner.Next(ctx, question, result, round)
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
		query = next
	}
	seen := map[string]bool{}
	unique := result[:0]
	for _, v := range result {
		if !seen[v.ID] {
			seen[v.ID] = true
			unique = append(unique, v)
		}
	}
	return unique, nil
}
