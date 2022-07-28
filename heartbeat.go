package heartbeat

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type HeartbeatConfig struct {
	interval time.Duration
	timeout time.Duration
}

func NewHeartBeatConfig(interval time.Duration, timeout time.Duration) HeartbeatConfig {
	return HeartbeatConfig{
		interval: interval,
		timeout: timeout,
	}
}

type Heartbeat struct {
	heartbeatChan chan interface{}
	ticker *time.Ticker
	lifecycle *LifecycleState
}

func NewHeartbeat(cfg HeartbeatConfig) *Heartbeat {
	ticker := time.NewTicker(cfg.interval)

	return newHeartbeat(ticker)
}

func newHeartbeat(ticker *time.Ticker) *Heartbeat {
	heartbeatChan := make(chan interface{})

	return &Heartbeat{
		heartbeatChan: heartbeatChan,
		ticker: ticker,
		lifecycle: NewLifecycleState(Started),
	}
}

func (h *Heartbeat) PulseInterval() <-chan time.Time {
	return h.ticker.C
}

func (h *Heartbeat) SendPulse() error {
	if h.lifecycle.State() == Stopped {
		return ErrAlreadyStopped
	}

	select {
	case h.heartbeatChan <- struct{}{}:
	default:
	}

	return nil
}

func (h *Heartbeat) Close() error {
	h.ticker.Stop()
	if err := h.lifecycle.Stop(); err != nil {
		return fmt.Errorf("failed to stop heartbeat: %w", err)
	}

	close(h.heartbeatChan)

	return nil
}

type HeartbeatMonitor struct {
	healthChan chan bool
	healthState int32
	heartbeat *Heartbeat
	lifecycle *LifecycleState

	ctx context.Context
	cancelFunc context.CancelFunc
	wg sync.WaitGroup

	timeout time.Duration

}

func NewHeartBeatMonitor(heartbeat *Heartbeat, cfg HeartbeatConfig) *HeartbeatMonitor {
	healthChan := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background()) //TODO pass the context? So we can timeout?

	return &HeartbeatMonitor{
		healthChan: healthChan,
		heartbeat: heartbeat,
		lifecycle: NewLifecycleState(Stopped),
		ctx: ctx,
		cancelFunc: cancel,
		timeout: cfg.timeout,
	}
}

func (m *HeartbeatMonitor) Start() error {
	if m.lifecycle.State() == Started {
		return ErrAlreadyStarted
	}

	m.wg.Add(1)
	m.controlLoop(m.ctx)

	if err := m.lifecycle.Start(); err != nil {
		return fmt.Errorf("failed to start heartbeat monitor: %w", err)
	}

	return nil
}

func (m *HeartbeatMonitor) Close() error {
	if m.lifecycle.State() == Stopped {
		return ErrAlreadyStopped
	}
	
	m.cancelFunc()
	m.wg.Wait()

	if err := m.lifecycle.Stop(); err != nil {
		return fmt.Errorf("failed to stop heartbeat monitor: %w", err)
	}
	
	return nil
}

func (m *HeartbeatMonitor) HealthChan() <-chan bool {
	return m.healthChan
}

func (m *HeartbeatMonitor) controlLoop(ctx context.Context) {
	go func() {
		defer m.wg.Done()
		defer close(m.healthChan)
		defer m.heartbeat.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.heartbeat.heartbeatChan:
				m.sendIsHealthy(true)
			case <-time.After(m.timeout):
				m.sendIsHealthy(false)
			}
		}
	}()
}

func (m *HeartbeatMonitor) sendIsHealthy(healthy bool) {
	val := atomic.LoadInt32(&m.healthState)
	
	if (val == 1 && healthy) || (val == 2 && !healthy) {
		// no status change, no need to send
		return
	} 


	if !atomic.CompareAndSwapInt32(&m.healthState, val, m.healthStateVal(healthy)) {
		panic("failed to change healthstate")
	}

	select {
	case m.healthChan <- healthy:
	case <-time.After(m.timeout):
		panic("heartbeat monitor failed to report health status")
	}
}

func (m *HeartbeatMonitor) healthStateVal(healhy bool) int32 {
	if healhy {
		return 1
	} else {
		return 2
	}
}
