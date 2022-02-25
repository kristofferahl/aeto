package eventsource

import (
	"sort"
)

type Stream struct {
	Id      string
	Commits []Commit
}

func (s Stream) Length() int64 {
	l := int64(0)
	for _, c := range s.Commits {
		l += int64(len(c.Events))
	}
	return l
}

func (s Stream) Events() (events []Event) {
	for _, c := range s.Commits {
		events = append(events, c.Events...)
	}
	sort.Slice(events[:], func(i, j int) bool {
		return events[i].EventSequence() < events[j].EventSequence()
	})
	return
}

type Commit struct {
	Id     string
	Events EventList

	// TODO: Add commit sequence for sorting like events
	// TODO: Add Timestamp string
}

type EventList []Event

// TODO: Add Event Timestamp
type Event interface {
	// StreamId returns the id of the aggregate referenced by the event
	StreamId() string

	// EventSequence returns the sequence number of this event
	EventSequence() int64

	// setStreamId sets the stream id
	setStreamId(s string)

	// setEventSequence sets the sequence number
	setEventSequence(s int64)
}

// Record provides the serialized representation of the event
type Record struct {
	// Data contains the event in serialized form
	Data []byte
}

type EventModel struct {
	// Id contains the stream id
	Id string

	// Sequence contains the event sequence number
	Sequence int64
}

func (m *EventModel) StreamId() string {
	return m.Id
}

func (m *EventModel) EventSequence() int64 {
	return m.Sequence
}

func (m *EventModel) setStreamId(s string) {
	m.Id = s
}

func (m *EventModel) setEventSequence(s int64) {
	m.Sequence = s
}
