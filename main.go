package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

type IdleTracker struct {
	active map[net.Conn]bool
	idle   time.Duration
	timer  *time.Timer
}

func NewIdleTracker(idle time.Duration) *IdleTracker {
	return &IdleTracker{
		active: make(map[net.Conn]bool),
		idle:   idle,
		timer:  time.NewTimer(idle),
	}
}

func (t *IdleTracker) Done() <-chan time.Time {
	return t.timer.C
}

func main() {
	var host = flag.String("host", "http://localhost", "host")
	var port = flag.String("port", "8080", "port")
	var timeInSeconds = flag.Int("time", 60, "time in seconds")
	var upstreamTimeout = flag.Int("upstream-timeout", 0, "maximum time to wait before the upstream server is online")
	flag.Parse()

	fmt.Println("tired-proxy")

	remote, err := url.Parse(*host)
	if err != nil {
		panic(err)
	}

	// Check if we need to wait for the upstream proxy to be online
	if *upstreamTimeout > 0 {
		fmt.Printf("Waiting %d seconds for upstream host to come online\n", *upstreamTimeout)
		wfp := NewWaitForPortCmd(remote, PortInUse, *upstreamTimeout)
		if err := wfp.Wait(); err != nil {
			panic(fmt.Errorf("error while waiting for upstream host to come online: %w", err))
		}
		fmt.Println("Upstream host came online")
	}

	idle := NewIdleTracker(time.Duration(*timeInSeconds) * time.Second)

	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			idle.timer.Reset(idle.idle)
			log.Println(r.URL)
			r.Host = remote.Host
			w.Header().Set("X-Ben", "Rad")
			p.ServeHTTP(w, r)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	http.HandleFunc("/", handler(proxy))

	fmt.Println("Starting server")

	go func() {
		<-idle.Done()
		fmt.Println("Idle time passed, shutting down server")
		os.Exit(0)
	}()

	err = http.ListenAndServe(fmt.Sprintf(":%s", *port), nil)
	if err != nil {
		panic(err)
	}
}
