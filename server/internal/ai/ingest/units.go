package ingest

import (
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/doc"
)

func Units(documentID string, value doc.Document) ([]retrieve.Unit, error) {
	blocks, err := doc.Blocks(value)
	if err != nil {
		return nil, err
	}
	result := make([]retrieve.Unit, 0, len(blocks))
	for _, b := range blocks {
		if b.Text == "" {
			continue
		}
		heading := ""
		if len(b.HeadingPath) > 0 {
			heading = b.HeadingPath[len(b.HeadingPath)-1]
		}
		result = append(result, retrieve.Unit{ID: b.ID, DocumentID: documentID, Heading: heading, Text: b.Text, Position: b.Position})
	}
	return result, nil
}
