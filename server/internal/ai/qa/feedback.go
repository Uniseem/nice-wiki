package qa

func Feedback(records *Records, id, conversation string, helpful bool) bool {
	return records != nil && records.Feedback(id, conversation, helpful)
}
