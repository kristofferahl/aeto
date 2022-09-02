package eventsource

type Repository interface {
	Get(streamId string) (stream Stream, err error)
	Save(aggregate Aggregate) (events int, err error)
	Delete(stream Stream) (err error)
}
