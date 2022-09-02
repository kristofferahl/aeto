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
		&ResourceNamespaceNameChanged{},
		&ResourceGenererationFailed{},
		&ResourceGenererationSuccessful{},
		&ResourceSetVersionChanged{},
		&ResourceSetCreated{},
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

	// Name is the name of the Tenant.
	Name string `json:"name"`

	// Namespace is the namespace where the Tenant was created.
	Namespace string `json:"namespace"`
}

type TenantDisplayNameSet struct {
	eventsource.EventModel
	Name string `json:"name"`
}

type BlueprintSet struct {
	eventsource.EventModel
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type LabelsChanged struct {
	eventsource.EventModel
	Labels map[string]string `json:"labels"`
}

type AnnotationsChanged struct {
	eventsource.EventModel
	Annotations map[string]string `json:"annotations"`
}

type ResourceNamespaceNameChanged struct {
	eventsource.EventModel
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
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

type ResourceSetCreated struct {
	eventsource.EventModel
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
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
