package reconcile

type GenericFinalizer struct {
	finalizerName string
	handlerFunc   func(Context) Result
}

func (f GenericFinalizer) Name() string {
	return f.finalizerName
}

func (f GenericFinalizer) Handler() func(Context) Result {
	return f.handlerFunc
}

func NewGenericFinalizer(finalizer string, handler func(Context) Result) GenericFinalizer {
	return GenericFinalizer{
		finalizerName: finalizer,
		handlerFunc:   handler,
	}
}
