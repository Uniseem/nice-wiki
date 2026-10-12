package api

import (
	"context"
	"errors"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/llm"
	"time"
)

func (s *Server) ListModels(w http.ResponseWriter, r *http.Request) {
	if s.Catalog == nil {
		writeJSON(w, []any{})
		return
	}
	writeJSON(w, s.Catalog.List())
}
func (s *Server) SaveModel(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.ModelInfo](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	capabilities := make([]llm.Capability, len(body.Capabilities))
	for index, value := range body.Capabilities {
		capabilities[index] = llm.Capability(value)
	}
	model := llm.Model{ID: body.Id, Name: body.Name, Channel: body.Channel, Capabilities: capabilities, InputPrice: float64(body.InputPrice), OutputPrice: float64(body.OutputPrice), Context: body.Context, MaxOutput: body.MaxOutput}
	if err := s.Catalog.Merge([]llm.Model{model}); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_model", body.Channel+":"+body.Id) {
		return
	}
	writeJSON(w, model)
}
func (s *Server) UpdateChannel(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.ChannelInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Channels.Update(id, body.Name, body.BaseURL, string(body.Protocol), body.ApiKey)
	if err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "update_channel", id) {
		return
	}
	writeJSON(w, value)
}
func (s *Server) DeleteChannel(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.Channels.Delete(id); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "delete_channel", id) {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) ProbeChannel(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.ProbeInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	model := llm.Model{ID: body.Model, Channel: id}
	chat, err := (llm.ProtocolRegistry{Channels: s.Channels, HTTP: &http.Client{Timeout: 30 * time.Second}}).Chat(model)
	if err != nil {
		s.validationOrFailure(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := llm.Probe(ctx, chat, body.Model); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, struct{}{})
}

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
