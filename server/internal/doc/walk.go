package doc

// Walk 深度优先遍历，返回 false 可跳过该节点的子树。
func Walk(root Node, visit func(Node, Node, int) bool) {
	var walk func(Node, Node, int)
	walk = func(node, parent Node, depth int) {
		if !visit(node, parent, depth) {
			return
		}
		for _, child := range node.Content {
			walk(child, node, depth+1)
		}
	}
	walk(root, Node{}, 0)
}
func FindBlock(root Node, id string) (Node, bool) {
	var result Node
	found := false
	Walk(root, func(node, parent Node, depth int) bool {
		if found {
			return false
		}
		if value, _ := node.Attrs["id"].(string); value == id {
			result = node
			found = true
			return false
		}
		return true
	})
	return result, found
}
