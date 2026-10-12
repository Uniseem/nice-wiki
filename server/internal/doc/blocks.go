package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"nice-wiki/internal/doc/nodes"
	"strings"
)

type Block struct {
	ID           string   `json:"id"`
	ParentID     string   `json:"parentId"`
	Kind         string   `json:"kind"`
	Position     int      `json:"position"`
	HeadingPath  []string `json:"headingPath"`
	Text         string   `json:"text"`
	ContentHash  string   `json:"contentHash"`
	Translatable []string `json:"translatable"`
}

func Blocks(document Document) ([]Block, error) {
	if err := Validate(document); err != nil {
		return nil, err
	}
	result := []Block{}
	headings := []string{}
	levels := []int{}
	var visit func(Node, string) error
	visit = func(node Node, parent string) error {
		if IsBlock(node.Type) {
			if node.Type == "heading" {
				level := nodes.Number(node, "level", 2)
				for len(levels) > 0 && levels[len(levels)-1] >= level {
					levels = levels[:len(levels)-1]
					headings = headings[:len(headings)-1]
				}
				levels = append(levels, level)
				headings = append(headings, nodes.InlineText(node))
			}
			encoded, err := json.Marshal(node)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(encoded)
			hash := hex.EncodeToString(digest[:])
			if err != nil {
				return err
			}
			text := nodes.InlineText(node)
			if node.Type == "blockMath" {
				text = nodes.String(node, "latex")
			}
			for _, key := range []string{"alt", "caption", "title", "description"} {
				if v := nodes.String(node, key); v != "" {
					text += " " + v
				}
			}
			translated := []string{}
			if renderer, ok := nodes.Lookup(node.Type); ok {
				translated = append(translated, renderer.Translate(node)...)
			}
			id := nodes.String(node, "id")
			result = append(result, Block{ID: id, ParentID: parent, Kind: node.Type, Position: len(result), HeadingPath: append([]string{}, headings...), Text: strings.TrimSpace(text), ContentHash: hash, Translatable: translated})
			parent = id
		}
		for _, child := range node.Content {
			if err := visit(child, parent); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(document.Content, ""); err != nil {
		return nil, err
	}
	return result, nil
}
