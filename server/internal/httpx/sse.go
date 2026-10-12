package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type SSE struct {
	writer http.ResponseWriter
	flush  http.Flusher
}

func NewSSE(w http.ResponseWriter) (*SSE, error) {
	flush, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("响应不支持流式刷新")
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	return &SSE{w, flush}, nil
}
func (s *SSE) Event(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return err
	}
	s.flush.Flush()
	return nil
}
func (s *SSE) Comment(value string) error {
	_, err := fmt.Fprintf(s.writer, ": %s\n\n", value)
	if err == nil {
		s.flush.Flush()
	}
	return err
}
