package card

import (
	"strings"
	"testing"
)

func TestGuideScopeAndContinuation(t *testing.T) {
	parent := "section"
	nodes := []treeNode{{DocumentID: parent, Title: "配置"}, {DocumentID: "outside", Title: "其他分区"}, {DocumentID: "one", Title: "第一篇", ParentID: &parent}, {DocumentID: "two", Title: "第二篇", ParentID: &parent}}
	first, next, err := renderAgentGuide(nodes, nil, parent, 0, 1)
	if err != nil || next == nil || *next != 1 || !strings.Contains(first, "第一篇") || strings.Contains(first, "第二篇") || strings.Contains(first, "其他分区") {
		t.Fatal("分区指南越界或分页丢失")
	}
	last, next, err := renderAgentGuide(nodes, nil, parent, 1, 1)
	if err != nil || next != nil || !strings.Contains(last, "第二篇") || strings.Contains(last, "第一篇") {
		t.Fatal("指南续读不完整")
	}
	if _, _, err := renderAgentGuide(nodes, nil, "withdrawn", 0, 1); err == nil {
		t.Fatal("不可见分区仍返回指南")
	}
}
