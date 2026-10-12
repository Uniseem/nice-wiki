package qa

func Feedback(records *Records, id string, helpful bool) bool {
	return records != nil && records.Feedback(id, helpful)
}
