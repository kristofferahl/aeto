package eventsource

type Aggregate interface {
	Id() string
	Version() int64
	Commit() Commit
}

type AggregateRoot struct {
	id                string
	version           int64
	lastEventSequence int64
	uncommitted       EventList

	handler EventHandler
}

func (a *AggregateRoot) WithHandler(handler EventHandler) *AggregateRoot {
	a.handler = handler
	return a
}

func (a *AggregateRoot) WithId(id string) *AggregateRoot {
	a.id = id
	return a
}

func (a *AggregateRoot) WithVersion(v int64) *AggregateRoot {
	a.version = v
	return a
}

func (a *AggregateRoot) LoadFromHistoricalEvents(stream Stream) *AggregateRoot {
	for _, e := range stream.Events() {
		a.applyToInternalState(e)
	}
	a.WithVersion(stream.Version())
	return a
}

func (a *AggregateRoot) Id() string {
	return a.id
}

func (a *AggregateRoot) Version() int64 {
	return a.version
}

func (a *AggregateRoot) Apply(e Event) {
	e.setTimestamp()
	e.setSequence(a.lastEventSequence + 1)
	a.applyToInternalState(e)
	a.uncommitted = append(a.uncommitted, e)
}

func (a *AggregateRoot) applyToInternalState(e Event) {
	a.handler.On(e)
	a.lastEventSequence = e.EventSequence()
}

func (a *AggregateRoot) CommitEvents(version int64, handler func(e Event)) {
	if len(a.uncommitted) > 0 {
		for _, e := range a.uncommitted {
			handler(e)
		}
		a.uncommitted = EventList{}
		a.version = version
	}
}
