package nodes

type tasklist struct{}

func init() { Register("taskList", tasklist{}); Register("taskItem", tasklist{}) }
func (tasklist) Render(n Node, c Context) (string, error) {
	if n.Type == "taskList" {
		if c.Format == HTML {
			return Wrap(n, c, "ul", "nw-task-list")
		}
		return Children(n, c)
	}
	value, err := Children(n, c)
	if err != nil {
		return "", err
	}
	if c.Format == HTML {
		checked := ""
		if Bool(n, "checked") {
			checked = " checked"
		}
		return "<li" + Anchor(n) + ` class="nw-task-item"><input type="checkbox" disabled` + checked + ` aria-label="任务状态"><div>` + value + "</div></li>", nil
	}
	prefix := "- [ ] "
	if Bool(n, "checked") {
		prefix = "- [x] "
	}
	return PrefixLines(value, prefix), nil
}
func (tasklist) Translate(n Node) []string { return nil }
