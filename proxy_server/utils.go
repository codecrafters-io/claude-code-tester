package proxy_server

import (
	"net/url"
	"os"
)

const (
	openRouterUrl      = "https://openrouter.ai"
	proxyListeningPort = "10000"
)

func mustGetOpenrouterApiKey() string {
	apiKey := os.Getenv("CODECRAFTERS_SECRET_OPENROUTER_API_KEY")
	if apiKey == "" {
		panic("Codecrafters Internal Error - CODECRAFTERS_SECRET_OPENROUTER_API_KEY environment variable not set")
	}
	return apiKey
}

func mustParseUrl(urlString string) *url.URL {
	targetUrl, err := url.Parse(urlString)
	if err != nil {
		panic("Codecrafters Internal Error - Failed to parse OpenRouter's URL")
	}
	return targetUrl
}
