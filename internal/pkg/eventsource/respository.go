package eventsource

type Repository interface {
	Get(streamId string) (Stream, error)
	Save(aggregate Aggregate) error
}
