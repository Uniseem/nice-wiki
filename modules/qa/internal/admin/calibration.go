package admin

import (
	"context"
	"math"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"sort"
)

type ReadCacheCalibrationInput struct {
	From  string             `json:"from"`
	Until string             `json:"until"`
	Space gen.EmbeddingSpace `json:"space"`
}
type ReadCacheCalibrationResult = gen.ReadCacheCalibrationResult

func ReadCacheCalibration(ctx context.Context, op OperationContext, input ReadCacheCalibrationInput) (ReadCacheCalibrationResult, error) {
	if err := qaruntime.RequireAdmin(op, "qa.manage"); err != nil {
		return ReadCacheCalibrationResult{}, err
	}
	start, err := platform.ParseTimestamp(input.From)
	if err != nil {
		return ReadCacheCalibrationResult{}, err
	}
	end, err := platform.ParseTimestamp(input.Until)
	if err != nil || !end.After(start) {
		return ReadCacheCalibrationResult{}, platform.NewError(platform.CodeValidation, false, "校准时间范围不合法")
	}
	tx, err := qaruntime.BeginRead(ctx, op)
	if err != nil {
		return ReadCacheCalibrationResult{}, err
	}
	defer tx.Close()
	policy, err := qaruntime.ReadPolicy(tx)
	if err != nil {
		return ReadCacheCalibrationResult{}, err
	}
	if policy.RetrievalSpace != input.Space {
		return ReadCacheCalibrationResult{}, platform.NewError(platform.CodeIntegrityError, false, "校准空间不是当前完整空间")
	}
	type sample struct {
		score float64
		bad   int
	}
	samples := make([]sample, 0)
	result := ReadCacheCalibrationResult{SimilarityBuckets: make([]int, 20), UnhelpfulBuckets: make([]int, 20), RecommendedThreshold: policy.SemanticThreshold}
	rows, err := tx.Tx.QueryContext(tx.Context, `SELECT similarity,unhelpful FROM cache_calibration WHERE space_id=? AND created_ns>=? AND created_ns<?`, input.Space.SpaceId, start.UnixNano(), end.UnixNano())
	if err != nil {
		return result, qaruntime.StorageError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var similarity float64
		var unhelpful int
		if err := rows.Scan(&similarity, &unhelpful); err != nil {
			return ReadCacheCalibrationResult{}, qaruntime.StorageError(err)
		}
		if math.IsNaN(similarity) || math.IsInf(similarity, 0) || similarity < 0 || similarity > 1 || (unhelpful != 0 && unhelpful != 1) {
			return ReadCacheCalibrationResult{}, platform.NewError(platform.CodeIntegrityError, false, "校准记录不合法")
		}
		bucket := min(19, int(similarity/0.05))
		result.SimilarityBuckets[bucket]++
		result.UnhelpfulBuckets[bucket] += unhelpful
		result.SampleCount++
		samples = append(samples, sample{similarity, unhelpful})
	}
	if err := rows.Err(); err != nil {
		return ReadCacheCalibrationResult{}, qaruntime.StorageError(err)
	}
	if result.SampleCount >= 100 {
		sort.Slice(samples, func(i, j int) bool { return samples[i].score < samples[j].score })
		remaining, bad := len(samples), 0
		for _, sample := range samples {
			bad += sample.bad
		}
		for index := 0; index < len(samples); {
			threshold := max(0.94, samples[index].score)
			next := index
			for next < len(samples) && samples[next].score < threshold {
				bad -= samples[next].bad
				remaining--
				next++
			}
			if remaining >= 100 && bad*100 <= remaining {
				result.RecommendedThreshold = float32(threshold)
				break
			}
			if next >= len(samples) {
				break
			}
			score := samples[next].score
			for next < len(samples) && samples[next].score == score {
				bad -= samples[next].bad
				remaining--
				next++
			}
			index = next
		}
	}

	return result, nil
}
