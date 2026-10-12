package site

import (
	"nice-wiki/internal/doc"
	"nice-wiki/internal/doc/nodes"
)

type tocEntry struct {
	ID    string
	Title string
	Level int
}

func tableOfContents(document doc.Document) []tocEntry {
	result := []tocEntry{}
	doc.Walk(document.Content, func(node, parent doc.Node, depth int) bool {
		if node.Type == "heading" {
			result = append(result, tocEntry{ID: "b-" + nodes.String(node, "id"), Title: nodes.InlineText(node), Level: nodes.Number(node, "level", 2)})
		}
		return true
	})
	return result
}
