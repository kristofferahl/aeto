package eventstore

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
)

type jsonEvent struct {
	Type string
	Data json.RawMessage
}

// JsonSerializer provides a simple serializer implementation
type JsonSerializer struct {
	eventTypes map[string]reflect.Type
}

// Register registers the specified events with the serializer; may be called more than once
func (j *JsonSerializer) Register(events ...eventsource.Event) {
	for _, event := range events {
		eventType, t := EventType(event)
		if _, ok := j.eventTypes[eventType]; !ok {
			j.eventTypes[eventType] = t
		}
	}
}

// MarshalEvent converts an event into its persistent type, Record
func (j *JsonSerializer) MarshalEvent(v eventsource.Event) (eventsource.Record, error) {
	eventType, _ := EventType(v)

	data, err := json.Marshal(v)
	if err != nil {
		return eventsource.Record{}, err
	}

	data, err = json.Marshal(jsonEvent{
		Type: eventType,
		Data: json.RawMessage(data),
	})
	if err != nil {
		return eventsource.Record{}, fmt.Errorf("unable to encode event")
	}

	return eventsource.Record{
		Data: data,
	}, nil
}

// UnmarshalEvent converts the persistent type, Record, into an Event instance
func (j *JsonSerializer) UnmarshalEvent(record eventsource.Record) (eventsource.Event, error) {
	wrapper := jsonEvent{}
	err := json.Unmarshal(record.Data, &wrapper)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal event")
	}

	t, ok := j.eventTypes[wrapper.Type]
	if !ok {
		return nil, fmt.Errorf("unbound event type, %v", wrapper.Type)
	}

	v := reflect.New(t).Interface()
	err = json.Unmarshal(wrapper.Data, v)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal event data into %#v", v)
	}

	return v.(eventsource.Event), nil
}

// NewSerializer constructs a new JsonSerializer and populates it with the specified events.
// Bind may be subsequently called to add more events.
func NewSerializer(events ...eventsource.Event) *JsonSerializer {
	serializer := &JsonSerializer{
		eventTypes: map[string]reflect.Type{},
	}
	serializer.Register(events...)
	return serializer
}

// EventType is a helper func that extracts the event type of the event along with the reflect.Type of the event.
// Primarily useful for serializers that need to understand how marshal and unmarshal instances of Event to a []byte
func EventType(event eventsource.Event) (string, reflect.Type) {
	t := reflect.TypeOf(event)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name(), t
}
