package translate

func Reuse(existing File, id, hash string, minimum int) (Block, bool) {
	value, ok := existing.Blocks[id]
	if !ok || value.SourceHash != hash || len(value.Text) < minimum {
		return Block{}, false
	}
	return value, true
}
