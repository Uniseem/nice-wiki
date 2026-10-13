package retrieve

type Unit struct {
	ID, DocumentID, Heading, Text, Context string
	Position                               int
	BlockIDs                               []string
	SourceHash                             string
}

func ExpandSection(hit Unit, all []Unit, limit int) []Unit {
	result := []Unit{}
	for _, v := range all {
		if v.DocumentID == hit.DocumentID && v.Heading == hit.Heading {
			result = append(result, v)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}
