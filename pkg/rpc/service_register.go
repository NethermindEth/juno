package rpc

type Dispatcher struct {
	handlers []HandleParamsResulter
}

func (us *Dispatcher) MethodName(h HandleParamsResulter) string {
	return h.Name()
}

func (us *Dispatcher) Handlers() []HandleParamsResulter {
	return us.handlers
}

func NewRPCDispatcher(handlers []HandleParamsResulter) *Dispatcher {
	return &Dispatcher{handlers: handlers}
}
