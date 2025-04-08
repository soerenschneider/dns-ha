package status

import (
	"github.com/soerenschneider/dns-ha/internal/conf"
)

const InitialStateName = "initial"

type Initial struct {
	currentStreak int
	lastState     string

	cfgHealthyStreak   int
	cfgUnhealthyStreak int
}

func NewUnknownState(opts conf.StatusConfig) *Initial {
	return &Initial{
		currentStreak:      opts.InitialHealthyStreak,
		cfgHealthyStreak:   opts.HealthyStreak,
		cfgUnhealthyStreak: opts.UnhealthyStreak,
	}
}

func (s *Initial) Name() string {
	return InitialStateName
}

func (s *Initial) Streak() int {
	return s.currentStreak
}

func (s *Initial) Healthy(state StateContext) {
	if s.lastState == UnhealthyStateName {
		s.currentStreak = s.cfgHealthyStreak
		s.lastState = HealthyStateName
	}

	s.currentStreak--
	if s.currentStreak <= 0 {
		state.SetState(newHealthy(s.cfgHealthyStreak, s.cfgUnhealthyStreak))
	}
}

func (s *Initial) Unhealthy(state StateContext) {
	if s.lastState == HealthyStateName {
		s.currentStreak = s.cfgUnhealthyStreak
		s.lastState = UnhealthyStateName
	}

	s.currentStreak--
	if s.currentStreak <= 0 {
		state.SetState(newUnhealthy(s.cfgHealthyStreak, s.cfgUnhealthyStreak))
	}
}

func (s *Initial) Error(state StateContext) {
}
