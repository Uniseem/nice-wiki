package retrieve

import "sort"

func Fuse(lists ...[]Hit) []Hit {
	scores := map[string]float64{}
	for _, list := range lists {
		for rank, hit := range list {
			scores[hit.ID] += 1 / float64(60+rank+1)
		}
	}
	result := make([]Hit, 0, len(scores))
	for id, score := range scores {
		result = append(result, Hit{id, score})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	return result
}
