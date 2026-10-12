package translate

type Segment struct {
	ID, Text string
	Heading  bool
}

func Batch(values []Segment, maximum int) [][]Segment {
	if maximum < 100 {
		maximum = 100
	}
	result := [][]Segment{}
	current := []Segment{}
	size := 0
	flush := func() {
		if len(current) > 0 {
			result = append(result, current)
			current = nil
			size = 0
		}
	}
	for _, value := range values {
		if value.Heading && size >= maximum/2 {
			flush()
		}
		if size+len(value.Text) > maximum && len(current) > 0 {
			flush()
		}
		current = append(current, value)
		size += len(value.Text)
	}
	flush()
	return result
}
