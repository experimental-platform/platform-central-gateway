package proxy

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/koding/websocketproxy"
)

// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// Proxy is the Central Gateway's customisable HTTP proxy backend
type Proxy struct {
	backend          *url.URL
	transport        http.RoundTripper
	websocketProxy   http.Handler
	WebsocketEnabled bool
}

func isWebsocket(req *http.Request) bool {
	if strings.ToLower(req.Header.Get("Upgrade")) != "websocket" ||
		!strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade") {
		return false
	}
	return true
}

// New creates a new Central Gateway proxy backend
func New(backend *url.URL) *Proxy {
	wsBackend := *backend
	wsBackend.Scheme = "ws"
	return &Proxy{
		backend:          backend,
		transport:        http.DefaultTransport,
		websocketProxy:   websocketproxy.NewProxy(&wsBackend),
		WebsocketEnabled: true,
	}
}

/*func transformRequest(req *http.Request) {
	newReq := *req
	newReq.U
}*/

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if p.WebsocketEnabled && isWebsocket(req) {
		// we don't use https explicitly, ssl termination is done here
		req.URL.Scheme = "ws"
		p.websocketProxy.ServeHTTP(rw, req)
		return
	}

	req.URL.Scheme = p.backend.Scheme
	req.URL.Host = p.backend.Host
	req.URL.Path = path.Join(p.backend.Path, req.URL.Path)

	for _, h := range hopHeaders {
		req.Header.Del(h)
	}

	// TODO retain prior proxy info
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	resp, err := p.transport.RoundTrip(req)
	if err != nil {
		log.Errorf("proxying '%s': %s\n", req.RequestURI, err.Error())
		rw.Header().Set("Content-Type", "text/html")
		rw.WriteHeader(http.StatusBadGateway)
		f, err := os.Open("/502.html")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		io.Copy(rw, f)
		return
	}

	for _, h := range hopHeaders {
		resp.Header.Del(h)
	}

	copyHeaders(rw.Header(), resp.Header)
	rw.WriteHeader(resp.StatusCode)
	io.Copy(rw, resp.Body)
}
