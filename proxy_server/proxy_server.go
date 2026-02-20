package proxy_server

import (
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
	server *http.Server
}

func newProxyServer() *proxyServer {
	targetUrl, _ := url.Parse("https://openrouter.ai")
	apiKey := mustGetOpenrouterApiKey()

	// Prepare the interceptor reverse proxy
	reverseProxy := httputil.NewSingleHostReverseProxy(targetUrl)
	reverseProxy.Director = nil
	reverseProxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetURL(targetUrl)
		req.Out.Header.Set("Authorization", "Bearer "+apiKey)
	}

	validator := &validationMiddleware{}

	validator.setEndPointsWithValidators(map[string][]ValidationFunc{
		"/api/v1/messages":         {modelValidator},
		"/api/v1/chat/completions": {modelValidator},
		"/api/v1/responses":        {modelValidator},
	})

	return &proxyServer{
		server: &http.Server{
			Addr:    "localhost:" + proxyListeningPort,
			Handler: validator.WrapProxy(reverseProxy),
		},
	}
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
