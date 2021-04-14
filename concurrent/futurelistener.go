package concurrent

type FutureListener interface {
	OperationCompleted(f Future)
}

type DefaultFutureListener struct {
}

func (l *DefaultFutureListener) OperationCompleted(f Future) {
}
