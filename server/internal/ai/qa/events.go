package qa

type Event struct {
	Type           string     `json:"type"`
	Text           string     `json:"text,omitempty"`
	Message        string     `json:"message,omitempty"`
	Code           string     `json:"code,omitempty"`
	Citations      []Citation `json:"citations,omitempty"`
	ConversationID string     `json:"conversationId,omitempty"`
	RecordID       string     `json:"recordId,omitempty"`
}
type Citation struct {
	DocumentID string `json:"documentId"`
	BlockID    string `json:"blockId"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	SourceHash string `json:"sourceHash,omitempty"`
}
