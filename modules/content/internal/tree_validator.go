package content

import (
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type EnsureTreeAcyclicInput struct {
	Documents []gen.DocumentMetadata `json:"documents"`
	MovingID  *Identifier            `json:"movingId"`
	ParentID  *Identifier            `json:"parentId"`
}

type EnsureTreeAcyclicResult struct {
	Valid       bool         `json:"valid"`
	Descendants []Identifier `json:"descendants"`
}

// EnsureTreeAcyclic 先核完整原树，再核移动整棵子树后的层级；不修改输入。
func EnsureTreeAcyclic(input EnsureTreeAcyclicInput) (EnsureTreeAcyclicResult, error) {
	index := make(map[Identifier]int, len(input.Documents))
	children := make(map[Identifier][]int, len(input.Documents))
	for i, document := range input.Documents {
		if _, found := index[document.DocumentId]; found {
			return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeIntegrityError, false, "文档树包含重复ID")
		}
		index[document.DocumentId] = i
	}
	for i, document := range input.Documents {
		if document.ParentId == nil {
			continue
		}
		if _, found := index[*document.ParentId]; !found {
			return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeIntegrityError, false, "文档树缺少父节点")
		}
		children[*document.ParentId] = append(children[*document.ParentId], i)
	}
	depths, err := documentTreeDepths(input.Documents, index)
	if err != nil {
		return EnsureTreeAcyclicResult{}, err
	}
	result := EnsureTreeAcyclicResult{Valid: true, Descendants: make([]Identifier, 0)}
	if input.MovingID == nil {
		return result, nil
	}
	movingIndex, found := index[*input.MovingID]
	if !found {
		return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeValidation, false, "拟移文档不存在")
	}
	parentDepth := 0
	if input.ParentID != nil {
		parentIndex, found := index[*input.ParentID]
		if !found {
			return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeValidation, false, "拟父文档不存在")
		}
		if *input.ParentID == *input.MovingID {
			return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeValidation, false, "文档不能移入自身")
		}
		parentDepth = depths[parentIndex]
	}
	subtreeHeight := 1
	pending := append([]int(nil), children[*input.MovingID]...)
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		document := input.Documents[current]
		if input.ParentID != nil && document.DocumentId == *input.ParentID {
			return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeValidation, false, "文档不能移入自身后代")
		}
		result.Descendants = append(result.Descendants, document.DocumentId)
		height := depths[current] - depths[movingIndex] + 1
		if height > subtreeHeight {
			subtreeHeight = height
		}
		pending = append(pending, children[document.DocumentId]...)
	}
	if parentDepth+subtreeHeight > 32 {
		return EnsureTreeAcyclicResult{}, platform.NewError(platform.CodeValidation, false, "移动后文档树超过32层")
	}
	sort.Strings(result.Descendants)
	return result, nil
}

// documentTreeDepths 沿父链作三色遍历，根为第一层，避免递归不合法长链。
func documentTreeDepths(documents []gen.DocumentMetadata, index map[Identifier]int) ([]int, error) {
	colors := make([]uint8, len(documents))
	depths := make([]int, len(documents))
	for i := range documents {
		if colors[i] == 2 {
			continue
		}
		path := make([]int, 0, 32)
		current := i
		baseDepth := 0
		for {
			if colors[current] == 2 {
				baseDepth = depths[current]
				break
			}
			if colors[current] == 1 {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "文档树包含环")
			}
			colors[current] = 1
			path = append(path, current)
			if documents[current].ParentId == nil {
				break
			}
			current = index[*documents[current].ParentId]
		}
		for j := len(path) - 1; j >= 0; j-- {
			baseDepth++
			if baseDepth > 32 {
				return nil, platform.NewError(platform.CodeValidation, false, "文档树超过32层")
			}
			depths[path[j]] = baseDepth
			colors[path[j]] = 2
		}
	}
	return depths, nil
}
