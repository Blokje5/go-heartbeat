package heartbeat

import "sync/atomic"


type Lifecycle int32

const (
	Stopped Lifecycle = iota
	Started
	Unhealthy
)

func (l Lifecycle) String() string {
	switch l {
	case Started:
		return "started"
	case Stopped:
		return "stopped"
	case Unhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}
	
type LifecycleState struct {
	lifecycle int32
}

func NewLifecycleState(startingState Lifecycle) *LifecycleState {
	state := int32(startingState) // copy to avoid reference to passed variable
	return &LifecycleState{
		lifecycle: state,
	}
}

func (l *LifecycleState) State() Lifecycle {
	val := atomic.LoadInt32(&l.lifecycle)
	return Lifecycle(val)
}

func (l *LifecycleState) Start() error {
	val := atomic.LoadInt32(&l.lifecycle)

	if !atomic.CompareAndSwapInt32(&l.lifecycle, val, int32(Started)) {
		return ErrFailedTransition
	}

	return nil
}

func (l *LifecycleState) Stop() error {
	val := atomic.LoadInt32(&l.lifecycle)

	if !atomic.CompareAndSwapInt32(&l.lifecycle, val, int32(Stopped)) {
		return ErrFailedTransition
	}

	return nil
}


func (l *LifecycleState) Unhealthy() error {
	val := atomic.LoadInt32(&l.lifecycle)

	if !atomic.CompareAndSwapInt32(&l.lifecycle, val, int32(Unhealthy)) {
		return ErrFailedTransition
	}

	return nil
}
