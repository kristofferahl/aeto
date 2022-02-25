package eventstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"

	eventv1alpha1 "github.com/kristofferahl/aeto/apis/event/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Repository struct {
	client.Client
	Log        logr.Logger
	Context    context.Context
	serializer eventsource.Serializer
}

func StreamId(s string) string {
	return strings.ReplaceAll(s, "/", "-")
}

func New(client client.Client, log logr.Logger, context context.Context, serializer eventsource.Serializer) eventsource.Repository {
	return Repository{
		Client:     client,
		Log:        log,
		Context:    context,
		serializer: serializer,
	}
}

func (r Repository) Get(streamId string) (eventsource.Stream, error) {
	chunks, err := r.getEventStreamChunks(streamId)
	if err != nil {
		r.Log.V(1).Error(err, "failed to fetch event stream chunks")
		return eventsource.Stream{}, err
	}
	r.Log.V(1).Info("event stream chunks fetched", "chunks", len(chunks.Items))

	stream, err := r.convertToEventStream(chunks, streamId)
	if err != nil {
		r.Log.V(1).Error(err, "failed to convert event stream chunks to event stream")
		return eventsource.Stream{}, err
	}
	r.Log.V(1).Info("event stream loaded", "commits", len(stream.Commits))

	return stream, nil
}

func (r Repository) Save(aggregate eventsource.Aggregate) error {
	commit := aggregate.Commit()
	chunk, err := r.convertToEventStreamChunk(commit, aggregate.Id())
	if err != nil {
		return err
	}
	count := len(chunk.Spec.Events)
	if count > 0 {
		err = r.Create(r.Context, &chunk, &client.CreateOptions{
			FieldManager: "aeto",
		})
		if err != nil {
			return err
		}
		r.Log.V(1).Info(fmt.Sprintf("Committed %d event(s) to %s, aggregate %s is at version %d\n", count, chunk.Name, aggregate.Id(), aggregate.Version()))
	} else {
		r.Log.V(1).Info(fmt.Sprintf("0 events to commit, aggregate %s is at version %d\n", aggregate.Id(), aggregate.Version()))
	}
	return nil
}

func (r Repository) getEventStreamChunks(streamId string) (eventv1alpha1.EventStreamChunkList, error) {
	var eventStreamChunks eventv1alpha1.EventStreamChunkList

	// TODO: Index field streamId
	// fs, err := fields.ParseSelector(fmt.Sprintf("spec.streamId==%s", ctx.Request.NamespacedName))
	// if err != nil {
	// 	return eventStreamChunks, err
	// }

	options := client.ListOptions{}
	//options.FieldSelector = fs
	options.Namespace = config.Operator.Namespace

	if err := r.List(r.Context, &eventStreamChunks, &options); err != nil {
		return eventv1alpha1.EventStreamChunkList{}, err
	}

	filtered := make([]eventv1alpha1.EventStreamChunk, 0)
	for _, c := range eventStreamChunks.Items {
		if c.Spec.StreamId == streamId {
			filtered = append(filtered, c)
		}
	}
	eventStreamChunks.Items = filtered
	return eventStreamChunks, nil
}

func (r Repository) convertToEventStream(list eventv1alpha1.EventStreamChunkList, id string) (eventsource.Stream, error) {
	commits := make([]eventsource.Commit, 0)
	for _, c := range list.Items {
		if c.Spec.StreamId != id {
			return eventsource.Stream{}, fmt.Errorf("wrong expected stream id for chunk (expected=%s, actual=%s)", id, c.Spec.StreamId)
		}

		commit := eventsource.Commit{
			Id:     c.Name,
			Events: make([]eventsource.Event, 0),
		}
		for _, e := range c.Spec.Events {
			event, err := r.serializer.UnmarshalEvent(eventsource.Record{
				Data: []byte(e.Raw),
			})
			if err != nil {
				return eventsource.Stream{}, err
			}
			commit.Events = append(commit.Events, event)
		}
		commits = append(commits, commit)
	}
	return eventsource.Stream{
		Id:      id,
		Commits: commits,
	}, nil
}

func (r Repository) convertToEventStreamChunk(commit eventsource.Commit, streamId string) (eventv1alpha1.EventStreamChunk, error) {
	chunk := eventv1alpha1.EventStreamChunk{
		ObjectMeta: metav1.ObjectMeta{
			Name:      commit.Id,
			Namespace: config.Operator.Namespace,
		},
		Spec: eventv1alpha1.EventStreamChunkSpec{
			StreamId: streamId,
		},
	}
	for _, e := range commit.Events {
		record, err := r.serializer.MarshalEvent(e)
		if err != nil {
			return eventv1alpha1.EventStreamChunk{}, err
		}
		chunk.Spec.Events = append(chunk.Spec.Events, eventv1alpha1.EventRecord{
			Raw: string(record.Data),
		})
	}
	return chunk, nil
}
