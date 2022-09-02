package eventsource

type EventReplayer interface {
	Replay(eventList EventList) ReplayResult
}

type ReplayResult struct {
	Error error
}

func (r ReplayResult) Failed() bool {
	return r.Error != nil
}

func Replay(handler EventHandler, eventList EventList) ReplayResult {
	for _, e := range eventList {
		handler.On(e)
	}
	return ReplayResult{
		Error: nil,
	}
}
