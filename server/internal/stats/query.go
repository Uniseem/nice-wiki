package stats

type Summary struct {
	Views, Visitors, AICrawls int64
	Popular                   []Daily
}

func Summarize(values []Daily, limit int) Summary {
	result := Summary{}
	for _, v := range values {
		result.Views += v.Views
		result.Visitors += v.Visitors
		result.AICrawls += v.AICrawls
	}
	for _, v := range values {
		inserted := false
		for i, current := range result.Popular {
			if v.Views > current.Views {
				result.Popular = append(result.Popular, nilDaily)
				copy(result.Popular[i+1:], result.Popular[i:])
				result.Popular[i] = v
				inserted = true
				break
			}
		}
		if !inserted {
			result.Popular = append(result.Popular, v)
		}
	}
	if len(result.Popular) > limit {
		result.Popular = result.Popular[:limit]
	}
	return result
}

var nilDaily Daily
