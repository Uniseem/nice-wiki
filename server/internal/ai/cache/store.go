package cache

import "time"

func NewEntry(question, answer, language, memory string, references map[string]string, ttl time.Duration) Entry {
	return Entry{Question: question, Answer: answer, Language: language, MemoryVersion: memory, References: references, Created: time.Now(), Expires: time.Now().Add(ttl)}
}
