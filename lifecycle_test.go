package heartbeat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLifecycle(t *testing.T) {
	assertions := assert.New(t)

	state := NewLifecycleState(Stopped)
	assertions.NotNil(state)

	err := state.Start()
	assertions.NoError(err)

	lifecycle := state.State()
	assertions.Equal(Started, lifecycle)

	err = state.Unhealthy()
	assertions.NoError(err)

	lifecycle = state.State()
	assertions.Equal(Unhealthy, lifecycle)

	err = state.Stop()
	assertions.NoError(err)

	lifecycle = state.State()
	assertions.Equal(Stopped, lifecycle)
}
