package tenant

import (
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
)

func Events() []eventsource.Event {
	return []eventsource.Event{
		&TenantCreated{},
		&TenantDisplayNameSet{},
		&BlueprintSet{},
		&LabelsChanged{},
		&AnnotationsChanged{},
		&ResourceGenererationFailed{},
		&ResourceGenererationSuccessful{},
		&ResourceSetVersionChanged{},
		&ResourceSetNameChanged{},
		&ResourceAdded{},
		&ResourceUpdated{},
		&ResourceRemoved{},
		&ResourceSetActivated{},
		&ResourceSetDeactivated{},
		&TenantDeleted{},
	}
}

type TenantCreated struct {
	eventsource.EventModel
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type TenantDisplayNameSet struct {
	eventsource.EventModel
	Name string `json:"name"`
}

type BlueprintSet struct {
	eventsource.EventModel
	Name string `json:"name"`
}

type LabelsChanged struct {
	eventsource.EventModel
	Labels map[string]string `json:"labels"`
}

type AnnotationsChanged struct {
	eventsource.EventModel
	Annotations map[string]string `json:"annotations"`
}

type ResourceGenererationFailed struct {
	eventsource.EventModel
	Sum string `json:"sum"`
}

type ResourceGenererationSuccessful struct {
	eventsource.EventModel
	Sum string `json:"sum"`
}

type ResourceSetVersionChanged struct {
	eventsource.EventModel
	Version int `json:"version"`
}

type ResourceSetNameChanged struct {
	eventsource.EventModel
	Name string `json:"name"`
}

type ResourceAdded struct {
	eventsource.EventModel
	Resource Resource `json:"resource"`
}

type ResourceUpdated struct {
	eventsource.EventModel
	Resource Resource `json:"resource"`
}

type ResourceRemoved struct {
	eventsource.EventModel
	ResourceId string `json:"resourceId"`
}

type ResourceSetActivated struct {
	eventsource.EventModel
	Name string `json:"name"`
}

type ResourceSetDeactivated struct {
	eventsource.EventModel
	Name string `json:"name"`
}

type TenantDeleted struct {
	eventsource.EventModel
}
