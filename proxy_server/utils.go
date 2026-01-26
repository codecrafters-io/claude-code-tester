package proxy_server

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

const (
	proxyListeningPort = "10000"
)

func mustGetOpenrouterApiKey() string {
	apiKey := os.Getenv("CODECRAFTERS_SECRET_OPENROUTER_API_KEY")
	if apiKey == "" {
		panic("Codecrafters Internal Error - CODECRAFTERS_SECRET_OPENROUTER_API_KEY environment variable not set")
	}
	return apiKey
}

func getSha256HashString(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}
