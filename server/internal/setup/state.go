package setup

import (
	"errors"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/settings"
)

type Channels interface{ Count() int }
type State struct {
	Registered  bool `json:"registered"`
	Initialized bool `json:"initialized"`
	AIEnabled   bool `json:"aiEnabled"`
}
type Service struct {
	Auth     *auth.Service
	Settings *settings.Store
	Channels Channels
}

func (s Service) State() State {
	return State{Registered: s.Auth.Registered(), Initialized: s.Settings.Site().Initialized, AIEnabled: s.Settings.AI().Enabled}
}
func (s Service) Complete() error {
	if !s.Auth.Registered() {
		return errors.New("请先注册超管")
	}
	if s.Channels.Count() == 0 {
		return errors.New("请先登记至少一个模型渠道")
	}
	return s.Settings.CompleteSetup()
}
