package proxy_server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

// StartProxyServer spawns a proxy that listens at localhost:10000
// the server is automatically shutdown as a part of stageHarness' teardown function
//
// The returned recorder holds every request the user's program sent. Stages
// that assert only on stdout can ignore it.
func StartProxyServer(stageHarness *test_case_harness.TestCaseHarness) *RequestRecorder {
	proxyServer, requestRecorder := newProxyServer()
	proxyServer.Start()
	proxyServer.registerTeardown(stageHarness)

	return requestRecorder
}

type proxyServer struct {
	server *http.Server
}

func newProxyServer() (*proxyServer, *RequestRecorder) {
	targetUrl, _ := url.Parse("https://openrouter.ai")
	apiKey := mustGetOpenrouterApiKey()

	// Prepare the interceptor reverse proxy
	reverseProxy := httputil.NewSingleHostReverseProxy(targetUrl)
	reverseProxy.Director = nil
	reverseProxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetURL(targetUrl)
		req.Out.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Disable "http: proxy error: context canceled" log
	reverseProxy.ErrorLog = log.Default()
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// Reference: https://github.com/golang/go/blob/acd65ebb13a11f1b070b63a66b35bb1b15934409/src/net/http/httputil/reverseproxy.go#L376
		// Reference2: https://github.com/golang/go/issues/20071#issuecomment-926644055
		// Keep the behavior same, except for context cancelled errors
		if !errors.Is(err, context.Canceled) {
			reverseProxy.ErrorLog.Printf("http: proxy error: %v", err)
		}
		w.WriteHeader(http.StatusBadGateway)
	}

	validator := &validationMiddleware{}

	validator.setEndPointsWithValidators(map[string][]ValidationFunc{
		"/api/v1/chat/completions": {modelValidator},
		"/api/v1/responses":        {modelValidator},
		// For Claude Code
		"/api/v1/messages": {modelValidator},
	})

	requestRecorder := &RequestRecorder{}

	// Recording sits outside validation, so a request is captured as the user's
	// program sent it whether or not the proxy goes on to reject it.
	return &proxyServer{
		server: &http.Server{
			Addr:    "localhost:" + proxyListeningPort,
			Handler: requestRecorder.WrapHandler(validator.WrapProxy(reverseProxy)),
		},
	}, requestRecorder
}

func (s *proxyServer) Start() {
	go s.listenAndServe()
	s.waitForServerStart()
}

// listenAndServe starts listening and serving on specified listening port
func (s *proxyServer) listenAndServe() {
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed to start proxy server: %s", err.Error()))
	}
}

// waitForServerStart tries to a establish connection to the proxy's listening port
// returns immediately after a successful connection has been established
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

// registerTeardown registers a function to close the proxy server
func (s *proxyServer) registerTeardown(stageHarness *test_case_harness.TestCaseHarness) {
	stageHarness.RegisterTeardownFunc(func() {
		if err := s.server.Close(); err != nil {
			panic("Codecrafters Internal Error - Failed closing proxy server: " + err.Error())
		}
	})
}
