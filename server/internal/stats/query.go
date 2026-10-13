package stats

import "sort"

type Summary struct {
	Views     int64       `json:"views"`
	Visitors  int64       `json:"visitors"`
	AICrawls  int64       `json:"aiCrawls"`
	Popular   []Popular   `json:"popular"`
	Languages []Breakdown `json:"languages"`
	Countries []Breakdown `json:"countries"`
	Referrers []Breakdown `json:"referrers"`
}

type Popular struct {
	Title string `json:"title"`
	Views int64  `json:"views"`
}
type Breakdown struct {
	Value    string `json:"value"`
	Views    int64  `json:"views"`
	Visitors int64  `json:"visitors"`
}

func Summarize(values []Daily, limit int) Summary {
	result := Summary{}
	hasTotal := false
	for _, v := range values {
		switch v.Dimension {
		case "total":
			hasTotal = true
			result.Views += v.Views
			result.Visitors += v.Visitors
		case "ai":
			result.AICrawls += v.Views
		case "language":
			result.Languages = append(result.Languages, Breakdown{Value: v.Value, Views: v.Views, Visitors: v.Visitors})
		case "country":
			result.Countries = append(result.Countries, Breakdown{Value: v.Value, Views: v.Views, Visitors: v.Visitors})
		case "referrer":
			result.Referrers = append(result.Referrers, Breakdown{Value: v.Value, Views: v.Views, Visitors: v.Visitors})
		}
	}
	for _, v := range values {
		if v.Dimension != "path" && v.Path == "" {
			continue
		}
		path := v.Value
		if path == "" {
			path = v.Path
		}
		if !hasTotal {
			result.Views += v.Views
			result.Visitors += v.Visitors
		}
		popular := Popular{Title: path, Views: v.Views}
		inserted := false
		for i, current := range result.Popular {
			if popular.Views > current.Views {
				result.Popular = append(result.Popular, Popular{})
				copy(result.Popular[i+1:], result.Popular[i:])
				result.Popular[i] = popular
				inserted = true
				break
			}
		}
		if !inserted {
			result.Popular = append(result.Popular, popular)
		}
	}
	if len(result.Popular) > limit {
		result.Popular = result.Popular[:limit]
	}
	sortBreakdowns := func(values []Breakdown) []Breakdown {
		sort.Slice(values, func(i, j int) bool { return values[i].Views > values[j].Views })
		if len(values) > limit {
			values = values[:limit]
		}
		return values
	}
	result.Languages = sortBreakdowns(result.Languages)
	result.Countries = sortBreakdowns(result.Countries)
	result.Referrers = sortBreakdowns(result.Referrers)
	return result
}
