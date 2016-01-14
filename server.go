package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/koding/websocketproxy"
)

var DEBUG = false
var if_bind *string
var apps_target *string
var management_target *string
var apps_proxy *SwitchingProxy
var management_proxy *httputil.ReverseProxy
var devices_proxy *httputil.ReverseProxy

var ADMIN_NAME = "admin"
var ADMIN_PATH = "/" + ADMIN_NAME
var ADMIN_FULL_PATH = ADMIN_PATH + "/"
var DEVICES_PATH = "/devices/"

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	if DEBUG {
		fmt.Printf("[%v] %+v\n", time.Now(), req)
	}
	urlPath := req.URL.Path
	if urlPath == "/" || urlPath == "" || urlPath == ADMIN_PATH {
		http.Redirect(w, req, ADMIN_FULL_PATH, http.StatusMovedPermanently)
	} else if strings.HasPrefix(req.URL.String(), ADMIN_FULL_PATH) {
		management_proxy.ServeHTTP(w, req)
	} else if strings.HasPrefix(req.URL.String(), DEVICES_PATH) {
		devices_proxy.ServeHTTP(w, req)
	} else {
		apps_proxy.ServeHTTP(w, req)
	}
}

type SwitchingProxy struct {
	httpProxy      http.Handler
	websocketProxy http.Handler
}

func newSwitchingProxy(backend *url.URL) *SwitchingProxy {
	wsBackend := *backend
	wsBackend.Scheme = "ws"
	return &SwitchingProxy{
		httpProxy:      httputil.NewSingleHostReverseProxy(backend),
		websocketProxy: websocketproxy.NewProxy(&wsBackend),
	}
}

func (p *SwitchingProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if isWebsocket(req) {
		// we don't use https explicitly, ssl termination is done here
		req.URL.Scheme = "ws"
		p.websocketProxy.ServeHTTP(rw, req)
		return
	}

	p.httpProxy.ServeHTTP(rw, req)
}

func isWebsocket(req *http.Request) bool {
	if strings.ToLower(req.Header.Get("Upgrade")) != "websocket" ||
		!strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade") {
		return false
	}
	return true
}

func main() {
	if_bind = flag.String("interface", "127.0.0.1:3001", "server interface to bind")
	apps_target = flag.String("apps", "http://127.0.0.1:8080", "target URL for apps reverse proxy")
	management_target = flag.String("management", "http://127.0.0.1:8081", "target URL for management reverse proxy")
	flag.Parse()

	fmt.Printf("Interface:      %v\n", *if_bind)
	fmt.Printf("Apps-Url:       %v\n", *apps_target)
	fmt.Printf("Management-Url: %v\n", *management_target)

	apps_target_url, _ := url.Parse(*apps_target)
	management_target_url, _ := url.Parse(*management_target)
	devices_target_url, _ := url.Parse("http://127.0.0.1:9200")

	apps_proxy = newSwitchingProxy(apps_target_url)
	management_proxy = httputil.NewSingleHostReverseProxy(management_target_url)
	devices_proxy = httputil.NewSingleHostReverseProxy(devices_target_url)

	go func() {
		signal_chan := make(chan os.Signal, 10)
		signal.Notify(signal_chan, syscall.SIGUSR1)
		for true {
			<-signal_chan
			DEBUG = !DEBUG
			fmt.Printf("Set debug to %v.\n", DEBUG)
		}
	}()

	http.HandleFunc("/", defaultHandler)
	err := http.ListenAndServe(*if_bind, nil)
	if err != nil {
		panic(err)
	}
}
