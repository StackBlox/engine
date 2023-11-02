package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

func GeneratePassword(length int) (string, error) {
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(randomBytes), nil
}

func GenerateSecureUsername(prefix string, length int) (string, error) {
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	randomPart := base64.StdEncoding.EncodeToString(randomBytes)
	for _, char := range []string{"/", "+", "="} {
		randomPart = strings.ReplaceAll(randomPart, char, "")
	}

	return fmt.Sprintf("%s%s", prefix, randomPart[:length]), nil
}
