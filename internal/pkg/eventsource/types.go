package eventsource

import (
	"sort"
	"time"
)

func NewStream(id string, commits ...Commit) Stream {
	return Stream{
		id:      id,
		commits: commits,
	}
}

type Stream struct {
	id      string
	commits []Commit
}

func (s Stream) Id() string {
	return s.id
}

func (s Stream) Length() int64 {
	l := int64(0)
	for _, c := range s.Commits() {
		l += int64(len(c.Events()))
	}
	return l
}

func (s Stream) Commits() []Commit {
	sort.Slice(s.commits[:], func(i, j int) bool {
		return s.commits[i].Sequence() < s.commits[j].Sequence()
	})
	return s.commits
}

func (s Stream) Version() int64 {
	if len(s.commits) == 0 {
		return 0
	}
	commits := s.Commits()
	return commits[len(commits)-1].Sequence()
}

func (s Stream) Events() (events []Event) {
	for _, c := range s.Commits() {
		events = append(events, c.Events()...)
	}
	sort.Slice(events[:], func(i, j int) bool {
		return events[i].EventSequence() < events[j].EventSequence()
	})
	return
}

func NewCommit(id string, sequence int64) *Commit {
	return &Commit{
		id:        id,
		sequence:  sequence,
		timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		events:    EventList{},
	}
}

type Commit struct {
	id        string
	timestamp string
	sequence  int64
	events    EventList
}

func (c *Commit) Id() string {
	return c.id
}

func (c *Commit) Timestamp() string {
	return c.timestamp
}

func (c *Commit) Sequence() int64 {
	return c.sequence
}

func (c *Commit) Events() EventList {
	return c.events
}

func (c *Commit) SetTimestamp(timestamp string) {
	c.timestamp = timestamp
}

func (c *Commit) Append(event Event) {
	c.events = append(c.events, event)
}

type EventList []Event

type Event interface {
	// EventSequence returns the sequence number of this event
	EventSequence() int64

	// EventTimestamp returns the timestamp of the event
	EventTimestamp() string

	// setSequence sets the sequence number
	setSequence(s int64)

	// setTimestamp sets the timestamp
	setTimestamp()
}

// Record provides the serialized representation of the event
type Record struct {
	// Data contains the event in serialized form
	Data []byte
}

type EventModel struct {
	// Sequence contains the event sequence number
	Sequence int64 `json:"seq"`

	// Timestamp contains the timestamp of the event
	Timestamp string `json:"ts"`
}

func (m *EventModel) EventSequence() int64 {
	return m.Sequence
}

func (m *EventModel) EventTimestamp() string {
	return m.Timestamp
}

func (m *EventModel) setSequence(s int64) {
	m.Sequence = s
}

func (m *EventModel) setTimestamp() {
	m.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
}
