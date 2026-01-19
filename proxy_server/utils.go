package proxy_server

import (
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
