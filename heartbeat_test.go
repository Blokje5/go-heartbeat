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
		cfg             HeartbeatConfig
		testRunDuration time.Duration
		runner          func(context.Context, *Heartbeat)
		healthEvaluator func(context.Context, <-chan bool, *assert.Assertions) chan interface{}
	}{
		{
			name:            "monitor should remain healthy with a well-behaving runner",
			cfg:             NewHeartBeatConfig(100*time.Millisecond, 200*time.Millisecond),
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
			healthEvaluator: func(ctx context.Context, c <-chan bool, assertions *assert.Assertions) chan interface{} {
				done := make(chan interface{})

				go func() {
					defer close(done)
					healthStatusSlice := make([]bool, 0)
					for {
						select {
						case healthy := <-c:
							healthStatusSlice = append(healthStatusSlice, healthy)
						case <-ctx.Done():
							assertions.ElementsMatch([]bool{true}, healthStatusSlice)
							return
						}
					}

				}()

				return done
			},
		},
		{
			name:            "monitor should remain send unhealthy once a heartbeat fails",
			cfg:             NewHeartBeatConfig(100*time.Millisecond, 200*time.Millisecond),
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
			healthEvaluator: func(ctx context.Context, c <-chan bool, assertions *assert.Assertions) chan interface{} {
				done := make(chan interface{})

				go func() {
					defer close(done)
					healthStatusSlice := make([]bool, 0)
					for {
						select {
						case healthy := <-c:
							healthStatusSlice = append(healthStatusSlice, healthy)
						case <-ctx.Done():
							assertions.ElementsMatch([]bool{true, false}, healthStatusSlice)
							return
						}
					}
				}()

				return done
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertions := assert.New(t)

			heartbeat := NewHeartbeat(tt.cfg)

			monitor := NewHeartBeatMonitor(heartbeat, tt.cfg)

			ctx, cancelFunc := context.WithCancel(context.Background())

			go tt.runner(ctx, heartbeat)

			err := monitor.Start()
			assertions.NoError(err)

			done := tt.healthEvaluator(ctx, monitor.HealthChan(), assertions)

			<-time.After(tt.testRunDuration)
			cancelFunc()

			err = monitor.Close()
			assertions.NoError(err)

			<-done
		})
	}
}
