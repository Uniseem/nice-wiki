package api

import (
	"errors"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/llm"
)

func (s *Server) channelResponse(v llm.Channel) gen.Channel {
	return gen.Channel{Id: v.ID, Name: v.Name, BaseURL: v.BaseURL, Protocol: gen.ChannelProtocol(v.Protocol), HasKey: s.Channels.HasKey(v.ID)}
}
func (s *Server) ListChannels(w http.ResponseWriter, r *http.Request) {
	values := s.Channels.List()
	result := make([]gen.Channel, len(values))
	for i, v := range values {
		result[i] = s.channelResponse(v)
	}
	writeJSON(w, result)
}
func (s *Server) CreateChannel(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.ChannelInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Channels.Create(body.Name, body.BaseURL, string(body.Protocol), body.ApiKey)
	if err != nil {
		badRequest(w, errors.New("渠道配置无效或保存失败"))
		return
	}
	if !s.audit(w, r, "create_channel", value.ID) {
		return
	}
	writeJSON(w, s.channelResponse(value))
}
