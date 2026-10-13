package translate

func reusableSource(source string, minimum int) bool {
	return len([]rune(placeholder.ReplaceAllString(source, ""))) >= minimum
}

func Reuse(existing File, id, hash, source string, minimum int) (Block, bool) {
	value, ok := existing.Blocks[id]
	if !ok || value.SourceHash != hash || !reusableSource(source, minimum) {
		return Block{}, false
	}
	return value, true
}
