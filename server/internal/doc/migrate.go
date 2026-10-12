package doc

import (
	"encoding/json"
	"fmt"
)

type Migration func(Document) (Document, error)

var migrations = map[int]Migration{}

func Migrate(data []byte) (Document, error) {
	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		return document, err
	}
	if document.SchemaVersion < 1 || document.SchemaVersion > SchemaVersion {
		return document, fmt.Errorf("不支持文档版本 %d", document.SchemaVersion)
	}
	for document.SchemaVersion < SchemaVersion {
		next, exists := migrations[document.SchemaVersion]
		if !exists {
			return document, fmt.Errorf("缺少版本 %d 的迁移", document.SchemaVersion)
		}
		previous := document.SchemaVersion
		var err error
		document, err = next(document)
		if err != nil {
			return document, err
		}
		if document.SchemaVersion != previous+1 {
			return document, fmt.Errorf("迁移没有前进一个版本")
		}
	}
	return document, Validate(document)
}
