package eventstore

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"

	eventv1alpha1 "github.com/kristofferahl/aeto/apis/event/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var StreamIdFieldIndexKey = "spec.streamId"

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
	r.Log.V(1).Info("event stream chunks fetched", "chunks", len(chunks))

	stream, err := r.convertToEventStream(chunks, streamId)
	if err != nil {
		r.Log.V(1).Error(err, "failed to convert event stream chunks to event stream")
		return eventsource.Stream{}, err
	}
	r.Log.V(1).Info("event stream loaded", "version", stream.Version())

	return stream, nil
}

func (r Repository) Save(aggregate eventsource.Aggregate) (events int, err error) {
	commit := aggregate.Commit()
	chunk, err := r.convertToEventStreamChunk(commit, aggregate.Id())
	if err != nil {
		return 0, err
	}
	count := len(chunk.Spec.Events)
	if count > 0 {
		err = r.Create(r.Context, &chunk, &client.CreateOptions{
			FieldManager: kubernetes.FieldManagerName,
		})
		if err != nil {
			return 0, err
		}
		r.Log.V(1).Info(fmt.Sprintf("Committed %d event(s) to %s, aggregate %s is at version %d", count, chunk.Name, aggregate.Id(), aggregate.Version()))
	} else {
		r.Log.V(1).Info(fmt.Sprintf("0 events to commit, aggregate %s is at version %d", aggregate.Id(), aggregate.Version()))
	}
	return count, nil
}

func (r Repository) Delete(stream eventsource.Stream) error {
	// TODO: Implement delete of EventStreamChunks using DeleteAllOf and FieldSelector
	commits := stream.Commits()
	sort.Slice(commits[:], func(i, j int) bool {
		return commits[i].Sequence() > commits[j].Sequence() // TODO: Verify delete order
	})

	deleted := 0
	for _, c := range commits {
		nn := types.NamespacedName{
			Namespace: config.Operator.Namespace,
			Name:      c.Id(),
		}
		r.Log.V(1).Info("deleting EventStreamChunk", "chunk", nn.String())
		if err := r.Client.Delete(r.Context, &eventv1alpha1.EventStreamChunk{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nn.Namespace,
				Name:      nn.Name,
			},
		}); client.IgnoreNotFound(err) != nil {
			r.Log.Error(err, "failed to delete EventStreamChunk", "chunk", nn.String())
			return err
		}
		deleted++
	}

	if deleted != len(commits) {
		return fmt.Errorf("not all EventStreamChunk(s) were deleted")
	}

	return nil
}

func (r Repository) getEventStreamChunks(streamId string) ([]eventv1alpha1.EventStreamChunk, error) {
	var eventStreamChunks eventv1alpha1.EventStreamChunkList

	fs, err := fields.ParseSelector(fmt.Sprintf("%s==%s", StreamIdFieldIndexKey, streamId))
	if err != nil {
		return eventStreamChunks.Items, err
	}

	options := client.ListOptions{}
	options.FieldSelector = fs
	options.Namespace = config.Operator.Namespace

	if err := r.List(r.Context, &eventStreamChunks, &options); err != nil {
		return make([]eventv1alpha1.EventStreamChunk, 0), err
	}

	return eventStreamChunks.Sort(), nil
}

func (r Repository) convertToEventStream(chunks []eventv1alpha1.EventStreamChunk, id string) (eventsource.Stream, error) {
	commits := make([]eventsource.Commit, 0)
	for _, c := range chunks {
		if c.Spec.StreamId != id {
			return eventsource.Stream{}, fmt.Errorf("wrong expected stream id for chunk (expected=%s, actual=%s)", id, c.Spec.StreamId)
		}

		commit := eventsource.NewCommit(c.Name, c.Spec.StreamVersion)
		commit.SetTimestamp(c.Spec.Timestamp)
		for _, e := range c.Spec.Events {
			event, err := r.serializer.UnmarshalEvent(eventsource.Record{
				Data: []byte(e.Raw),
			})
			if err != nil {
				return eventsource.Stream{}, err
			}
			commit.Append(event)
		}
		commits = append(commits, *commit)
	}
	return eventsource.NewStream(id, commits...), nil
}

func (r Repository) convertToEventStreamChunk(commit eventsource.Commit, streamId string) (eventv1alpha1.EventStreamChunk, error) {
	chunk := eventv1alpha1.EventStreamChunk{
		ObjectMeta: metav1.ObjectMeta{
			Name:      commit.Id(),
			Namespace: config.Operator.Namespace,
		},
		Spec: eventv1alpha1.EventStreamChunkSpec{
			StreamId:      streamId,
			StreamVersion: commit.Sequence(),
			Timestamp:     commit.Timestamp(),
		},
	}
	for _, e := range commit.Events() {
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
