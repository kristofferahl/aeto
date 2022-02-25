package eventsource

// Serializer converts between Events and Records
type Serializer interface {
	// MarshalEvent converts an Event to a Record
	MarshalEvent(event Event) (Record, error)

	// UnmarshalEvent converts an Event backed into a Record
	UnmarshalEvent(record Record) (Event, error)
}
