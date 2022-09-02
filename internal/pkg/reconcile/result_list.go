package reconcile

// ResultList contains a list of results
type ResultList []Result

// AllDone returns true when all result represents a successful and completed reconcile attempt
func (rl ResultList) AllDone() bool {
	for _, rr := range rl {
		if rr.RequiresRequeue() {
			return false
		}
	}
	return true
}

// AllSuccessful returns true when all result represents a successful reconcile attempt
func (rl ResultList) AllSuccessful() bool {
	for _, rr := range rl {
		if rr.Error() {
			return false
		}
	}
	return true
}
