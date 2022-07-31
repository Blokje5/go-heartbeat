package heartbeat

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Config struct {
	interval time.Duration
	timeout  time.Duration
}

func NewConfig(interval time.Duration, timeout time.Duration) Config {
	return Config{
		interval: interval,
		timeout:  timeout,
	}
}

type Heartbeat struct {
	heartbeatChan chan interface{}
	ticker        *time.Ticker
	lifecycle     *LifecycleState
}

func New(cfg Config) *Heartbeat {
	ticker := time.NewTicker(cfg.interval)

	return newHeartbeat(ticker)
}

func newHeartbeat(ticker *time.Ticker) *Heartbeat {
	heartbeatChan := make(chan interface{})

	return &Heartbeat{
		heartbeatChan: heartbeatChan,
		ticker:        ticker,
		lifecycle:     NewLifecycleState(Started),
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

type HealthListener interface {
	ShouldTrigger(healthy bool) bool
	Trigger(healthy bool)
}

type healthListenerHandler struct {
	shouldTrigger func(healthy bool) bool
	trigger func(healthy bool)
}

func HealthListenerFunction(shouldTrigger func(healthy bool) bool, trigger func(healthy bool)) HealthListener {
	return &healthListenerHandler{
		shouldTrigger: shouldTrigger,
		trigger: trigger,
	}
}

func (h *healthListenerHandler) ShouldTrigger(healthy bool) bool {
	return h.shouldTrigger(healthy)
}

func (h *healthListenerHandler) Trigger(healthy bool) {
	h.trigger(healthy)
}

type Monitor struct {
	healthState bool
	mutex sync.Mutex
	cond sync.Cond

	heartbeat   *Heartbeat
	lifecycle   *LifecycleState

	listeners 	[]HealthListener

	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	timeout time.Duration
}

func NewMonitor(heartbeat *Heartbeat, cfg Config) *Monitor {
	ctx, cancel := context.WithCancel(context.Background()) //TODO pass the context? So we can timeout?

	return &Monitor{
		heartbeat:  heartbeat,
		cond:	   sync.Cond{L: &sync.Mutex{}},
		lifecycle:  NewLifecycleState(Stopped),
		ctx:        ctx,
		cancelFunc: cancel,
		timeout:    cfg.timeout,
	}
}

func (m *Monitor) RegisterListener(listener HealthListener) *Monitor {
	m.listeners = append(m.listeners, listener)
	
	return m
}

func (m *Monitor) startListener(listener HealthListener) {
	go func() {
		m.cond.L.Lock()
		defer m.cond.L.Unlock()
		for !listener.ShouldTrigger(m.isHealthy()) {
			m.cond.Wait()
		}
		listener.Trigger(m.isHealthy())
	}()
}

func (m *Monitor) Start() error {
	if m.lifecycle.State() == Started {
		return ErrAlreadyStarted
	}

	for _, l := range m.listeners {
		m.startListener(l)
	}

	m.wg.Add(1)
	m.controlLoop(m.ctx)

	if err := m.lifecycle.Start(); err != nil {
		return fmt.Errorf("failed to start heartbeat monitor: %w", err)
	}

	return nil
}

func (m *Monitor) Close() error {
	if m.lifecycle.State() == Stopped {
		return ErrAlreadyStopped
	}

	m.cancelFunc()
	m.wg.Wait() // TODO do I need a wg here?

	if err := m.lifecycle.Stop(); err != nil {
		return fmt.Errorf("failed to stop heartbeat monitor: %w", err)
	}

	return nil
}

func (m *Monitor) controlLoop(ctx context.Context) {
	go func() {
		defer m.wg.Done()
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

func (m *Monitor) sendIsHealthy(healthy bool) {
	m.cond.L.Lock()
	defer m.cond.L.Unlock()
	state := m.healthState

	if (state && healthy) || (!state && !healthy) {
		// no status change, no need to send
		return
	}

	m.healthState = healthy
	m.cond.Broadcast()
}

func (m *Monitor) isHealthy() bool {
	return m.healthState
}

func (m *Monitor) healthStateVal(healhy bool) int32 {
	if healhy {
		return 1
	} else {
		return 2
	}
}
