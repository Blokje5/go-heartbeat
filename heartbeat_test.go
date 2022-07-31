package heartbeat

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHeartbeatMonitor(t *testing.T) {
	tests := []struct {
		name            string
		cfg             Config
		testRunDuration time.Duration
		runner          func(context.Context, *Heartbeat)
		healthEvaluator func([]bool, *assert.Assertions)
	}{
		{
			name:            "monitor should remain healthy with a well-behaving runner",
			cfg:             NewConfig(100*time.Millisecond, 200*time.Millisecond),
			testRunDuration: 1 * time.Second,
			runner: func(ctx context.Context, h *Heartbeat) {
				for {
					select {
					case <-h.PulseInterval():
						if err := h.SendPulse(); err != nil {
							return
						}
					case <-ctx.Done():
						return
					}
				}
			},
			healthEvaluator: func(healthStatusSlice []bool, assertions *assert.Assertions) {
				assertions.ElementsMatch([]bool{true}, healthStatusSlice)
			},
		},
		{
			name:            "monitor should remain send unhealthy once a heartbeat fails",
			cfg:             NewConfig(100*time.Millisecond, 200*time.Millisecond),
			testRunDuration: 1 * time.Second,
			runner: func(ctx context.Context, h *Heartbeat) {
				heartbeatCount := 0
				for heartbeatCount <= 2 { //stop running after a few heartbeats
					select {
					case <-h.PulseInterval():
						if err := h.SendPulse(); err != nil {
							return
						}
						heartbeatCount++
					case <-ctx.Done():
						return
					}
				}
			},
			healthEvaluator: func(healthStatusSlice []bool, assertions *assert.Assertions) {
				assertions.ElementsMatch([]bool{true, false}, healthStatusSlice)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertions := assert.New(t)

			heartbeat := New(tt.cfg)
			healthStatusSlice := make([]bool, 0)

			shouldTrigger := func(healthy bool) bool {
				return true
			}

			trigger := func (healthy bool) {
				healthStatusSlice= append(healthStatusSlice, healthy)
			}

			monitor := NewMonitor(heartbeat, tt.cfg)
			monitor.RegisterListener(HealthListenerFunction(shouldTrigger, trigger))

			ctx, cancelFunc := context.WithCancel(context.Background())

			go tt.runner(ctx, heartbeat)

			err := monitor.Start()
			assertions.NoError(err)

			
			<-time.After(tt.testRunDuration)
			cancelFunc()
			
			err = monitor.Close()
			assertions.NoError(err)
			
			tt.healthEvaluator(healthStatusSlice, assertions)
		})
	}
}
