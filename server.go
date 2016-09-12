package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/koding/websocketproxy"
)

const enableDokkuGateway bool = false

var DEBUG = false
var if_bind *string
var apps_target *string
var management_target *string
var apps_proxy *SwitchingProxy
var management_proxy *httputil.ReverseProxy
var devices_proxy *httputil.ReverseProxy
var soulNginxProxy *SwitchingProxy

var gatewayAppMap *hostToProxyMap

var ADMIN_NAME = "admin"
var ADMIN_PATH = "/" + ADMIN_NAME
var ADMIN_FULL_PATH = ADMIN_PATH + "/"
var DEVICES_PATH = "/devices/"

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	if DEBUG {
		fmt.Printf("[%v] %+v\n", time.Now(), req)
	}

	if enableDokkuGateway {
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
	} else {
		if appProxy := gatewayAppMap.matchHost(req.Host); appProxy != nil {
			appProxy.ServeHTTP(w, req)
			return
		}

		// default backend
		soulNginxProxy.ServeHTTP(w, req)
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

func createProxy() *goproxy.ProxyHttpServer {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	proxy.NonproxyHandler = http.HandlerFunc(defaultHandler)

	connectCondition := goproxy.ReqConditionFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) bool {
		return r.Method == "CONNECT"
	})

	proxy.OnRequest(connectCondition).HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
		sshConn, err := net.Dial("tcp", "127.0.0.1:22")
		if err != nil {
			client.Write([]byte("HTTP/1.1 500 Failed to connect to SSH\r\n\r\n"))
			//http.Error(client, err.Error(), http.StatusInternalServerError)
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			io.Copy(client, sshConn)
			wg.Done()
		}()
		go func() {
			io.Copy(sshConn, client)
			wg.Done()
		}()
		wg.Wait()
	})

	return proxy
}

func main() {
	var err error
	if_bind = flag.String("interface", "127.0.0.1:3001", "server interface to bind")
	apps_target = flag.String("apps", "http://127.0.0.1:8080", "target URL for apps reverse proxy")
	management_target = flag.String("management", "http://127.0.0.1:8081", "target URL for management reverse proxy")
	flag.Parse()

	if enableDokkuGateway {
		fmt.Printf("Interface:      %v\n", *if_bind)
		fmt.Printf("Apps-Url:       %v\n", *apps_target)
		fmt.Printf("Management-Url: %v\n", *management_target)

		apps_target_url, _ := url.Parse(*apps_target)
		management_target_url, _ := url.Parse(*management_target)
		devices_target_url, _ := url.Parse("http://127.0.0.1:9200")

		apps_proxy = newSwitchingProxy(apps_target_url)
		management_proxy = httputil.NewSingleHostReverseProxy(management_target_url)
		devices_proxy = httputil.NewSingleHostReverseProxy(devices_target_url)
	} else {
		soulNginxProxy, err = createSwitchingProxyToContainer("soul-nginx", 80)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		gatewayAppMap = &hostToProxyMap{}
		proxyCount, err := gatewayAppMap.reload()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("%d app proxy entries loaded\n", proxyCount)
	}

	proxy := createProxy()

	go func() {
		trafficEndpoint := "0.0.0.0:80"
		fmt.Printf("Listening at %s\n", trafficEndpoint)
		err := http.ListenAndServe(trafficEndpoint, proxy)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		controlEndpoint := "127.0.0.1:81"
		fmt.Printf("Control endpoint listening at %s\n", controlEndpoint)
		err := http.ListenAndServe(controlEndpoint, getControlHandler())
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		fmt.Printf("Listening (TLS)\n")
		err := http.ListenAndServeTLS("0.0.0.0:443", "/data/ssl/pem", "/data/ssl/key", proxy)
		if err != nil {
			panic(err)
		}
	}()

	signal_chan := make(chan os.Signal, 10)
	signal.Notify(signal_chan, syscall.SIGUSR1)
	for true {
		<-signal_chan
		DEBUG = !DEBUG
		fmt.Printf("Set debug to %v.\n", DEBUG)
	}
}

func createReverseProxyToContainer(containerName string, port uint16) (*httputil.ReverseProxy, error) {
	containerIP, err := getAppIP(containerName)
	if err != nil {
		return nil, err
	}

	url, err := url.Parse(fmt.Sprintf("http://%s:%d/", containerIP, port))
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(url), nil
}

func createSwitchingProxyToContainer(containerName string, port uint16) (*SwitchingProxy, error) {
	containerIP, err := getAppIP(containerName)
	if err != nil {
		return nil, err
	}

	url, err := url.Parse(fmt.Sprintf("http://%s:%d/", containerIP, port))
	if err != nil {
		return nil, err
	}

	return newSwitchingProxy(url), nil
}
