package status

type State interface {
	Name() string

	Streak() int
	Healthy(ctx StateContext)
	Unhealthy(ctx StateContext)
	Error(ctx StateContext)
}

type StateContext interface {
	SetState(state State)
}
