package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

func StringEnvVar(key, defaultValue string) string {
	v := os.Getenv(key)
	if v != "" {
		return v
	}
	return defaultValue
}

func BoolEnvVar(key string, defaultValue bool) bool {
	v := StringEnvVar(key, strconv.FormatBool(defaultValue))
	pv, err := strconv.ParseBool(v)
	if err != nil {
		log.Panicf("failed parsing boolean from environment variable %s", key)
	}
	return pv
}

func DurationEnvVar(key string, defaultValue time.Duration) time.Duration {
	v := StringEnvVar(key, defaultValue.String())
	pv, err := time.ParseDuration(v)
	if err != nil {
		log.Panicf("failed parsing duration from environment variable %s", key)
	}
	return pv
}
