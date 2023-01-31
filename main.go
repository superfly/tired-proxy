package main

import (
	"context"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var Version string
var log *logrus.Entry

func init() {
	base := logrus.New()
	base.Formatter = new(prefixed.TextFormatter)
	log = base.WithFields(logrus.Fields{"prefix": "tired-proxy"})
}

func main() {
	log.Info("Tired proxy - version ", Version)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var host = flag.String("host", "http://localhost", "host")
	var port = flag.String("port", "8080", "port")
	var timeInSeconds = flag.Int("time", 60, "time in seconds")
	var upstreamTimeout = flag.Int("upstream-timeout", 0, "maximum time to wait before the upstream server is online")
	flag.Parse()

	remote, err := url.Parse(*host)
	if err != nil {
		panic(err)
	}

	// Check if we need to wait for the upstream proxy to be online
	if *upstreamTimeout > 0 {
		log.Info("Waiting %d seconds for upstream host to come online\n", *upstreamTimeout)
		wfp := NewWaitForPortCmd(remote, PortInUse, *upstreamTimeout)
		if err := wfp.Wait(); err != nil {
			log.Panicf("error while waiting for upstream host to come online: %s", err)
		}
		log.Info("Upstream host came online")
	}

	log.Debug("Starting server")

	proxy := StartProxy(ctx, remote, *port, time.Duration(*timeInSeconds)*time.Second)

	// Setting up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// wait for either: the proxy to stop or the application to exit
	select {
	case err := <-proxy.Done():
		if err != nil {
			log.Errorf("Proxy error: %s", err)
		}
		log.Info("proxy done")
	case <-stop:
		log.Info("signal stop")
		cancel()
		t := time.Now()
		log.Debug("Stop proxy...")
		err := <-proxy.Done() // wait for the proxy to exit
		if err != nil {
			log.Errorf("Error: %s", err)
		}
		log.Debug("Stopped after: %s\n", time.Since(t))
		log.Info("signal: proxy done")
	}
	log.Info("exit")
}
