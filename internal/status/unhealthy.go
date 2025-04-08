package status

const UnhealthyStateName = "unhealthy"

type Unhealthy struct {
	currentStreak int

	cfgStreakUntilHealthy   int
	cfgStreakUntilUnhealthy int
}

func newUnhealthy(healthyStreak, unhealthyStreak int) *Unhealthy {
	return &Unhealthy{
		currentStreak:           unhealthyStreak,
		cfgStreakUntilHealthy:   healthyStreak,
		cfgStreakUntilUnhealthy: unhealthyStreak,
	}
}

func (s *Unhealthy) Name() string {
	return UnhealthyStateName
}

func (s *Unhealthy) Streak() int {
	return s.currentStreak
}

func (s *Unhealthy) Healthy(state StateContext) {
	s.currentStreak--
	if s.currentStreak <= 0 {
		state.SetState(newHealthy(s.cfgStreakUntilHealthy, s.cfgStreakUntilUnhealthy))
	}
}

func (s *Unhealthy) Unhealthy(state StateContext) {
	// reset streak
	s.currentStreak = s.cfgStreakUntilUnhealthy
}

func (s *Unhealthy) Error(state StateContext) {
	// reset streak
	s.currentStreak = s.cfgStreakUntilUnhealthy
}
