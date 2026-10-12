package cache

type Sample struct {
	Similarity float64
	Helpful    bool
}

func SuggestThreshold(values []Sample) float64 {
	best, score := 0.85, -1.0
	for threshold := 0.60; threshold <= 0.98; threshold += 0.01 {
		tp, fp, fn := 0, 0, 0
		for _, v := range values {
			hit := v.Similarity >= threshold
			if hit && v.Helpful {
				tp++
			}
			if hit && !v.Helpful {
				fp++
			}
			if !hit && v.Helpful {
				fn++
			}
		}
		denominator := 2*tp + fp + fn
		if denominator == 0 {
			continue
		}
		f1 := float64(2*tp) / float64(denominator)
		if f1 > score {
			score = f1
			best = threshold
		}
	}
	return best
}
