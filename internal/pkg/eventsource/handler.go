package eventsource

type EventHandler interface {
	On(e Event)
}
