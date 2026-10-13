package api

import (
	"context"
	"errors"
	"net/http"
	"nice-wiki/internal/aitask"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/jobs"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/settings"
	"slices"
	"strings"
	"time"
)

func modelReference(channel, id string) string { return channel + ":" + id }

func tierResponse(values map[string][]string) gen.ModelTiers {
	convert := func(tier string) []gen.ModelRef {
		result := []gen.ModelRef{}
		for _, value := range values[tier] {
			parts := strings.SplitN(value, ":", 2)
			if len(parts) == 2 {
				result = append(result, gen.ModelRef{Channel: parts[0], Id: parts[1]})
			}
		}
		return result
	}
	return gen.ModelTiers{Fast: convert(string(aitask.Fast)), Smart: convert(string(aitask.Smart)), Vision: convert(string(aitask.Vision)), Embedding: convert(string(aitask.Embedding)), Rerank: convert(string(aitask.Rerank))}
}

func (s *Server) validateModelTiers(body gen.ModelTiers) (map[string][]string, error) {
	models := map[string]llm.Model{}
	for _, model := range llm.AvailableModels(s.Catalog, s.Channels) {
		models[modelReference(model.Channel, model.ID)] = model
	}
	protocols := llm.ProtocolRegistry{Channels: s.Channels, HTTP: http.DefaultClient}
	values := map[string][]gen.ModelRef{
		string(aitask.Fast): body.Fast, string(aitask.Smart): body.Smart, string(aitask.Vision): body.Vision,
		string(aitask.Embedding): body.Embedding, string(aitask.Rerank): body.Rerank,
	}
	result := map[string][]string{}
	for tier, refs := range values {
		for _, ref := range refs {
			key := modelReference(ref.Channel, ref.Id)
			model, ok := models[key]
			if !ok {
				return nil, &settings.ValidationError{Message: "档位引用的模型不存在"}
			}
			var err error
			switch aitask.Tier(tier) {
			case aitask.Fast, aitask.Smart:
				if !model.Has(llm.Chat) {
					return nil, &settings.ValidationError{Message: "快速和精确档只能使用对话模型"}
				}
				_, err = protocols.Chat(model)
			case aitask.Vision:
				if !model.Has(llm.Chat) || !model.Has(llm.Vision) {
					return nil, &settings.ValidationError{Message: "视觉档只能使用支持看图的对话模型"}
				}
				_, err = protocols.Chat(model)
			case aitask.Embedding:
				if !model.Has(llm.Embedding) {
					return nil, &settings.ValidationError{Message: "嵌入档只能使用嵌入模型"}
				}
				_, err = protocols.Embedder(model)
			case aitask.Rerank:
				if !model.Has(llm.Rerank) {
					return nil, &settings.ValidationError{Message: "重排档只能使用重排模型"}
				}
				_, err = protocols.Reranker(model)
			}
			if err != nil {
				return nil, &settings.ValidationError{Message: "档位模型的渠道不可用"}
			}
			result[tier] = append(result[tier], key)
		}
	}
	return result, nil
}

func (s *Server) GetModelTiers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, tierResponse(s.Settings.ModelTiers()))
}

func (s *Server) SaveModelTiers(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.ModelTiers](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	before := s.Settings.ModelTiers()
	values, err := s.validateModelTiers(body)
	if err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if err := s.Settings.SaveModelTiers(values); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !slices.Equal(before[string(aitask.Embedding)], values[string(aitask.Embedding)]) && s.ClearAnswerCacheData != nil {
		if err := s.ClearAnswerCacheData(r.Context()); err != nil {
			s.fail(w, err)
			return
		}
	}
	if !s.audit(w, r, "save_model_tiers", "") {
		return
	}
	writeJSON(w, tierResponse(s.Settings.ModelTiers()))
}

func (s *Server) ListModels(w http.ResponseWriter, r *http.Request) {
	if s.Catalog == nil {
		writeJSON(w, []any{})
		return
	}
	writeJSON(w, llm.AvailableModels(s.Catalog, s.Channels))
}
func (s *Server) RefreshModelCatalog(w http.ResponseWriter, r *http.Request) {
	if s.Jobs == nil {
		writeError(w, http.StatusServiceUnavailable, "jobs_unavailable", "后台任务尚未配置")
		return
	}
	if err := s.Jobs.Enqueue(r.Context(), jobs.UpdateCatalog, "models.dev"); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, struct{}{})
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
	found := false
	for _, channel := range s.Channels.List() {
		found = found || channel.ID == model.Channel
	}
	if !found {
		s.validationOrFailure(w, &settings.ValidationError{Message: "模型渠道不存在"})
		return
	}
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
	value, err := s.Channels.Update(id, body.Name, body.Provider, body.BaseURL, string(body.Protocol), body.ApiKey, body.Enabled, body.TimeoutSeconds, body.StreamTimeoutSeconds, body.Concurrency, body.Headers)
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
	return gen.Channel{Id: v.ID, Name: v.Name, Provider: v.Provider, BaseURL: v.BaseURL, Protocol: gen.ChannelProtocol(v.Protocol), HasKey: s.Channels.HasKey(v.ID), Enabled: !v.Disabled, TimeoutSeconds: v.TimeoutSeconds, StreamTimeoutSeconds: v.StreamTimeoutSeconds, Concurrency: v.Concurrency, HeaderNames: append([]string{}, v.HeaderNames...)}
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
	headers := map[string]string{}
	if body.Headers != nil {
		headers = *body.Headers
	}
	value, err := s.Channels.Create(body.Name, body.Provider, body.BaseURL, string(body.Protocol), body.ApiKey, body.Enabled, body.TimeoutSeconds, body.StreamTimeoutSeconds, body.Concurrency, headers)
	if err != nil {
		badRequest(w, errors.New("渠道配置无效或保存失败"))
		return
	}
	if !s.audit(w, r, "create_channel", value.ID) {
		return
	}
	writeJSON(w, s.channelResponse(value))
}
