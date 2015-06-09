package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

var DEBUG = false
var if_bind *string
var apps_target *string
var management_target *string
var apps_proxy *httputil.ReverseProxy
var management_proxy *httputil.ReverseProxy

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	if DEBUG {
		fmt.Printf("[%v] %+v\n", time.Now(), req)
	}
	ext := path.Ext(req.URL.Path)
	if last := len(req.URL.Path); ext == "" && req.URL.Path[last-1:] != "/" {
		url := req.URL.Path + "/" // Always redirect with trailing slash
		http.Redirect(w, req, url, http.StatusMovedPermanently)
	} else if strings.HasPrefix(req.URL.String(), "/admin/") {
		management_proxy.ServeHTTP(w, req)
	} else {
		apps_proxy.ServeHTTP(w, req)
	}
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
	err := http.ListenAndServe(*if_bind, nil)
	if err != nil {
		panic(err)
	}
}
