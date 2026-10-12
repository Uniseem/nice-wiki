package history

import (
	"context"
	"encoding/json"
)

type Difference struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

func prettyJSON(data []byte) string {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return string(data)
	}
	formatted, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(formatted)
}

func (r *Repo) Diff(ctx context.Context, before, after, file string) (Difference, error) {
	oldValue, err := r.Read(ctx, before, file)
	if err != nil {
		return Difference{}, err
	}
	newValue, err := r.Read(ctx, after, file)
	if err != nil {
		return Difference{}, err
	}
	return Difference{Before: prettyJSON(oldValue), After: prettyJSON(newValue)}, nil
}
