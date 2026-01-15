package proxy_server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

// StartProxyServer spawns a proxy that listens at localhost:10000
// the server is automatically shutdown as a part of stageHarness' teardown function
func StartProxyServer(stageHarness *test_case_harness.TestCaseHarness) {
	proxyServer := newProxyServer()
	proxyServer.Start()
	proxyServer.registerTeardown(stageHarness)
}

type proxyServer struct {
	targetUrl *url.URL
	apiKey    string
	server    *http.Server
}

func newProxyServer() *proxyServer {
	apiKey := mustGetOpenrouterApiKey()
	targetUrl := mustParseUrl(openRouterUrl)

	return &proxyServer{
		targetUrl: targetUrl,
		apiKey:    apiKey,
	}
}

func (s *proxyServer) Start() {
	proxy := httputil.NewSingleHostReverseProxy(s.targetUrl)
	proxy.Director = nil

	interceptor := requestInterceptor{
		targetUrl: s.targetUrl,
		apiKey:    s.apiKey,
	}

	// intercept method modifies the outgoing request
	proxy.Rewrite = interceptor.intercept

	// validator wraps the proxy and returns error in case of invalid requests
	validator := validator{
		allowedModelPrefix: "anthropic/claude-haiku",
	}

	s.server = &http.Server{
		Addr:    ":" + proxyListeningPort,
		Handler: validator.wrapProxy(proxy),
	}

	go s.listenAndServe()

	s.waitForServerStart()
}

func (s *proxyServer) listenAndServe() {
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed to start proxy server: %s", err.Error()))
	}
}

func (s *proxyServer) waitForServerStart() {
	for {
		conn, _ := net.Dial("tcp", "localhost:"+proxyListeningPort)
		if conn != nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *proxyServer) registerTeardown(stageHarness *test_case_harness.TestCaseHarness) {
	stageHarness.RegisterTeardownFunc(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			stageHarness.Logger.Infof("Error shutting down proxy server: %v", err)
		}
	})
}
