package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var DEBUG = false
var port *int
var apps_target *string
var management_target *string
var apps_proxy *httputil.ReverseProxy
var management_proxy *httputil.ReverseProxy

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	if DEBUG {
		fmt.Printf("[%v] %+v\n", time.Now(), req)
	}
	if last := len(req.URL.Path); req.URL.Path[last-1:] != "/" {
		req.URL.Path = req.URL.Path + "/"
	}
	if strings.HasPrefix(req.URL.String(), "/protonet/") {
		management_proxy.ServeHTTP(w, req)
	} else {
		apps_proxy.ServeHTTP(w, req)
	}
}

func main() {
	port = flag.Int("port", 3001, "server port")
	apps_target = flag.String("apps", "http://localhost:8080", "target URL for apps reverse proxy")
	management_target = flag.String("management", "http://localhost:8081", "target URL for management reverse proxy")
	flag.Parse()

	fmt.Printf("Port:           %v\n", *port)
	fmt.Printf("Apps-Url:       %v\n", *apps_target)
	fmt.Printf("Management-Url: %v\n", *management_target)

	apps_target_url, _ := url.Parse(*apps_target)
	management_target_url, _ := url.Parse(*management_target)

	apps_proxy = httputil.NewSingleHostReverseProxy(apps_target_url)
	management_proxy = httputil.NewSingleHostReverseProxy(management_target_url)

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
	http.ListenAndServe(":"+strconv.Itoa(*port), nil)
}
