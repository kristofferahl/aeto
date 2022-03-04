package tenant

import (
	"fmt"

	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
)

type TenantAggregate struct {
	root  eventsource.AggregateRoot
	state State
}

type State struct {
	TenantName    string
	BlueprintName string
}

func NewTenant(id string) *TenantAggregate {
	a := &TenantAggregate{
		root:  eventsource.AggregateRoot{},
		state: State{},
	}
	a.root.
		WithId(id).
		WithVersion(0).
		WithHandler(&a.state)
	return a
}

func NewTenantFromEvents(stream eventsource.Stream) *TenantAggregate {
	a := NewTenant(stream.Id)
	a.root.
		LoadFromHistoricalEvents(stream).
		WithVersion(int64(len(stream.Commits))) // TODO: This could be a bad idea, better to use commit sequence number?
	return a
}

func (a *TenantAggregate) SetName(name string) {
	if a.state.TenantName != name {
		a.root.Apply(&TenantNameSet{Name: name})
	}
}

func (a *TenantAggregate) SetBlueprintName(name string) {
	if a.state.BlueprintName != name {
		a.root.Apply(&BlueprintSet{Name: name})
	}
}

func (a *TenantAggregate) Id() string {
	return a.root.Id()
}

func (a *TenantAggregate) Version() int64 {
	return a.root.Version()
}

func (a *TenantAggregate) Commit() eventsource.Commit {
	commit := eventsource.Commit{}
	a.root.CommitEvents(func(e eventsource.Event) {
		commit.Events = append(commit.Events, e)
	})
	commit.Id = fmt.Sprintf("%s-stream-chunk-%06d", a.root.Id(), a.root.Version())
	return commit
}

func (s *State) On(e eventsource.Event) {
	switch event := e.(type) {
	case *TenantNameSet:
		s.TenantName = event.Name
		break
	case *BlueprintSet:
		s.BlueprintName = event.Name
		break
	}
}
