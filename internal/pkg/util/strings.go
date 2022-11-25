package util

// SliceContainsString returns true when s is found in the slice
func SliceContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// SliceRemoveString removes s from the slice and returns a new slice without s
func SliceRemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// IndexOfString returns the index of the needle in the haystack
func IndexOfString(needle string, haystack []string) int {
	for k, v := range haystack {
		if needle == v {
			return k
		}
	}
	return -1
}
