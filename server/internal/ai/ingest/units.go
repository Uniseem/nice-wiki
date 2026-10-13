package ingest

import (
	"nice-wiki/internal/ai/retrieve"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/textutil"
	"strings"
)

func Units(documentID string, value doc.Document) ([]retrieve.Unit, error) {
	blocks, err := doc.Blocks(value)
	if err != nil {
		return nil, err
	}
	result := make([]retrieve.Unit, 0, len(blocks))
	current := retrieve.Unit{}
	hashes := []string{}
	flush := func() {
		if current.ID == "" {
			return
		}
		current.Text = strings.TrimSpace(current.Text)
		current.SourceHash = textutil.Hash([]byte(strings.Join(hashes, "\x00")))
		result = append(result, current)
		current = retrieve.Unit{}
		hashes = nil
	}
	for _, b := range blocks {
		if b.Text == "" {
			continue
		}
		heading := ""
		if len(b.HeadingPath) > 0 {
			heading = b.HeadingPath[len(b.HeadingPath)-1]
		}
		media := b.Kind == "image" || b.Kind == "drawing" || b.Kind == "attachment" || b.Kind == "video"
		if media {
			flush()
			result = append(result, retrieve.Unit{ID: b.ID, DocumentID: documentID, Heading: heading, Text: b.Text, Position: b.Position, BlockIDs: []string{b.ID}, SourceHash: b.ContentHash})
			continue
		}
		if current.ID != "" && (current.Heading != heading || len([]rune(current.Text))+len([]rune(b.Text)) > 800 && len([]rune(current.Text)) >= 300) {
			flush()
		}
		if current.ID == "" {
			current = retrieve.Unit{ID: b.ID, DocumentID: documentID, Heading: heading, Position: b.Position}
		}
		if current.Text != "" {
			current.Text += "\n"
		}
		current.Text += b.Text
		current.BlockIDs = append(current.BlockIDs, b.ID)
		hashes = append(hashes, b.ContentHash)
	}
	flush()
	return result, nil
}
