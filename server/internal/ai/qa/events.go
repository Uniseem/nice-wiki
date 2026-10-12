package qa

type Event struct {
	Type           string     `json:"type"`
	Text           string     `json:"text,omitempty"`
	Message        string     `json:"message,omitempty"`
	Citations      []Citation `json:"citations,omitempty"`
	ConversationID string     `json:"conversationId,omitempty"`
}
type Citation struct{ DocumentID, BlockID, Title, URL string }
