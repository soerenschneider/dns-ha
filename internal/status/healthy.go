package status

const HealthyStateName = "healthy"

type Healthy struct {
	currentStreak int

	cfgStreakUntilHealthy   int
	cfgStreakUntilUnhealthy int
}

func newHealthy(healthyStreak, unhealthyStreak int) *Healthy {
	return &Healthy{
		currentStreak:           healthyStreak,
		cfgStreakUntilHealthy:   healthyStreak,
		cfgStreakUntilUnhealthy: unhealthyStreak,
	}
}

func (s *Healthy) Name() string {
	return HealthyStateName
}

func (s *Healthy) Streak() int {
	return s.currentStreak
}

func (s *Healthy) Healthy(state StateContext) {
	// reset streak
	s.currentStreak = s.cfgStreakUntilUnhealthy
}

func (s *Healthy) Unhealthy(state StateContext) {
	s.currentStreak--
	if s.currentStreak <= 0 {
		state.SetState(newUnhealthy(s.cfgStreakUntilHealthy, s.cfgStreakUntilUnhealthy))
	}
}

func (s *Healthy) Error(state StateContext) {
	s.currentStreak--
	if s.currentStreak <= 0 {
		state.SetState(newUnhealthy(s.cfgStreakUntilHealthy, s.cfgStreakUntilUnhealthy))
	}
}
