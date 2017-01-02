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

func (proxy *Proxy) ServeHTTP(clientResponseWriter http.ResponseWriter, clientRequest *http.Request) {
	if proxy.WebsocketEnabled && isWebsocket(clientRequest) {
		// we don't use https explicitly, ssl termination is done here
		clientRequest.URL.Scheme = "ws"
		proxy.websocketProxy.ServeHTTP(clientResponseWriter, clientRequest)
		return
	}

	clientRequest.URL.Scheme = proxy.backend.Scheme
	clientRequest.URL.Host = proxy.backend.Host
	clientRequest.URL.Path = path.Join(proxy.backend.Path, clientRequest.URL.Path)

	for _, h := range hopHeaders {
		clientRequest.Header.Del(h)
	}

	// TODO retain prior proxy info
	// TODO: find out what 'prior proxy info' means

	// give more sense to tcpdump output ;)
	// TODO: add hostname and software version
	clientRequest.Header.Add("Via", "XXX (central-gateway, development version)")

	if clientIP, _, err := net.SplitHostPort(clientRequest.RemoteAddr); err == nil {
		clientRequest.Header.Set("X-Forwarded-For", clientIP)
	}

	// Retain SSL information.
	protocol := "http"
	if clientRequest.TLS != nil {
		protocol = "https"
	}
	clientRequest.Header.Set("X-Forwarded-Proto", protocol)

	// the actual proxying is going on here!
	serverResponse, err := proxy.transport.RoundTrip(clientRequest)

	if err != nil {
		log.Errorf("proxying '%s': %s\n", clientRequest.RequestURI, err.Error())
		clientResponseWriter.Header().Set("Content-Type", "text/html")
		clientResponseWriter.WriteHeader(http.StatusBadGateway)
		f, err := os.Open("/502.html")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		io.Copy(clientResponseWriter, f)
		return
	}

	for _, h := range hopHeaders {
		serverResponse.Header.Del(h)
	}
	// replace server software, so tcpdump on the external connection (and wget -S) makes more sense.
	serverResponse.Header.Set("Server", "central-gateway")
	copyHeaders(clientResponseWriter.Header(), serverResponse.Header)
	clientResponseWriter.WriteHeader(serverResponse.StatusCode)
	io.Copy(clientResponseWriter, serverResponse.Body)
}
