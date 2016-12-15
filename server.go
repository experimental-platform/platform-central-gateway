package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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

	"github.com/experimental-platform/platform-central-gateway/proxy"
	skvs "github.com/experimental-platform/platform-skvs/client"

	"github.com/elazarl/goproxy"
)

const enableDokkuGateway bool = false

var DEBUG = false
var if_bind *string
var apps_target *string
var management_target *string
var apps_proxy http.Handler
var management_proxy *httputil.ReverseProxy
var devices_proxy *httputil.ReverseProxy
var soulNginxProxy http.Handler

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

func getSSLCert(c *skvs.Client) (string, string, error) {
	if c == nil {
		var err error
		c, err = skvs.NewFromDocker()
		if err != nil {
			return "", "", fmt.Errorf("getSSLCert: %s", err.Error())
		}
	}

	pemData, pemErr := c.Get("ssl/pem")
	keyData, keyErr := c.Get("ssl/key")
	if keyErr != nil || pemErr != nil {
		fmt.Println("Failed to load certificate from SKVS - defaulting to pre-generated self-signed.")
		return "/data/ssl/pem", "/data/ssl/key", nil
	}

	err := ioutil.WriteFile("/tmp/gateway_pem", []byte(pemData), 0644)
	if err != nil {
		return "", "", fmt.Errorf("getSSLCert: %s", err.Error())
	}
	err = ioutil.WriteFile("/tmp/gateway_key", []byte(keyData), 0644)
	if err != nil {
		return "", "", fmt.Errorf("getSSLCert: %s", err.Error())
	}

	fmt.Println("Loaded TLS certificate from SKVS")
	return "/tmp/gateway_pem", "/tmp/gateway_key", nil
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

		apps_proxy = proxy.New(apps_target_url)
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
		pemPath, keyPath, err := getSSLCert(nil)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Listening (TLS)\n")
		err = http.ListenAndServeTLS("0.0.0.0:443", pemPath, keyPath, proxy)
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

func createSwitchingProxyToContainer(containerName string, port uint16) (http.Handler, error) {
	containerIP, err := getAppIP(containerName)
	if err != nil {
		return nil, err
	}

	url, err := url.Parse(fmt.Sprintf("http://%s:%d/", containerIP, port))
	if err != nil {
		return nil, err
	}

	return proxy.New(url), nil
}
