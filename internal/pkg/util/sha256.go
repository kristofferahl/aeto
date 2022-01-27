package util

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func AsSha256(o interface{}) (string, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum), nil
}

func Sha256Equal(a, b interface{}) (bool, error) {
	aSum, err := AsSha256(a)
	if err != nil {
		return false, err
	}
	bSum, err := AsSha256(b)
	if err != nil {
		return false, err
	}
	return aSum == bSum, nil
}
