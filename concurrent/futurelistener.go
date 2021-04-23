package concurrent

type FutureListener interface {
	OperationCompleted(f Future)
}

type DefaultFutureListener struct {
}

func (l *DefaultFutureListener) OperationCompleted(f Future) {
}

type InlineFutureListener struct {
	DefaultFutureListener
	f func(f Future)
}

func (l *InlineFutureListener) OperationCompleted(f Future) {
	l.f(f)
}

func NewFutureListener(f func(f Future)) FutureListener {
	return &InlineFutureListener{
		f: f,
	}
}
