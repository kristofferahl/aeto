package config

func NonLoggableKinds() []string {
	return []string{
		"Secret",
	}
}
