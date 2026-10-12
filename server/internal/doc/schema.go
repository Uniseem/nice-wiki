package doc

import "nice-wiki/internal/doc/nodes"

const SchemaVersion = 1
const MaxDocumentBytes = 8 << 20
const MaxNodes = 20000
const MaxDepth = 64

type Node = nodes.Node
type Mark = nodes.Mark

type Document struct {
	SchemaVersion int  `json:"schemaVersion"`
	Content       Node `json:"content"`
}

// 所有第一版节点均提前声明，复杂节点的渲染按计划后续接入。
var NodeNames = []string{
	"doc", "text", "hardBreak", "paragraph", "heading", "blockquote", "horizontalRule",
	"bulletList", "orderedList", "listItem", "taskList", "taskItem", "callout",
	"table", "tableRow", "tableCell", "tableHeader", "codeBlock", "inlineMath", "blockMath",
	"image", "attachment", "drawing", "video", "columns", "column",
}
var MarkNames = []string{"bold", "italic", "underline", "strike", "code", "highlight", "link", "internalLink"}
