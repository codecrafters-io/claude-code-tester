package proxy_server

import (
	"net/http/httputil"
	"net/url"
)

type requestInterceptor struct {
	targetUrl *url.URL
	apiKey    string
}

func (ri *requestInterceptor) intercept(req *httputil.ProxyRequest) {
	ri.setRequestUrlAndHost(req)
	ri.injectApiKey(req)
}

func (ri *requestInterceptor) setRequestUrlAndHost(req *httputil.ProxyRequest) {
	req.SetURL(ri.targetUrl)
	req.Out.Host = ri.targetUrl.Host
}

func (ri *requestInterceptor) injectApiKey(req *httputil.ProxyRequest) {
	req.Out.Header.Set("Authorization", "Bearer "+ri.apiKey)
}
