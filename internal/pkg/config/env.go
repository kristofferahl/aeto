package config

import (
	"fmt"
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

func IntEnvVar(key string, defaultValue int) int {
	v := StringEnvVar(key, fmt.Sprintf("%d", defaultValue))
	pv, err := strconv.Atoi(v)
	if err != nil {
		log.Panicf("failed parsing int from environment variable %s", key)
	}
	return pv
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
