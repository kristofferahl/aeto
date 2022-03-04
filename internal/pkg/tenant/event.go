package tenant

import (
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
)

func Events() []eventsource.Event {
	return []eventsource.Event{
		&TenantNameSet{},
		&BlueprintSet{},
	}
}

type TenantNameSet struct {
	eventsource.EventModel
	Name string
}

type BlueprintSet struct {
	eventsource.EventModel
	Name string
}
