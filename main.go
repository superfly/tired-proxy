package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
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
	// make all other packages also log with logrus

	stdlog.SetOutput(log.Writer())
}

func main() {
	fmt.Printf("Tired proxy - version %s\n", Version)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var origin = flag.String("origin", "http://localhost", "the origin host to which the requests are forwarded")
	var port = flag.String("port", "8080", "port at which the proxy server listens for requests")
	var idleTime = flag.Int("idle-time", 60, "idle time in seconds after which the application shuts down, if no requests where received")
	var waitFortPortTime = flag.Int("wait-for-port", 0, "maximum time in seconds to wait before the origin servers port is in use before starting the proxy server")
	var verbose = flag.Bool("verbose", false, "verbose logging output")
	flag.Parse()

	if *verbose {
		log.Logger.SetLevel(logrus.DebugLevel)
	}

	originUrl, err := url.Parse(*origin)
	if err != nil {
		log.Panicf("Invalid url given as host parameter: %s", err)
	}

	// Check if we need to wait for the origin server to be online
	if *waitFortPortTime > 0 {
		log.Infof("Waiting %d seconds for upstream host to come online", *waitFortPortTime)
		wfp := NewWaitForPortCmd(originUrl, PortInUse, *waitFortPortTime)
		if err := wfp.Wait(); err != nil {
			log.Panicf("error while waiting for upstream host to come online: %s", err)
		}
		log.Info("Upstream host came online")
	}

	log.Debug("About to start proxy server...")

	proxy := StartIdleProxy(ctx, originUrl, *port, time.Duration(*idleTime)*time.Second)

	// Setting up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// wait for either: the proxy to stop or the application to exit
	select {
	case err := <-proxy.Done():
		if err != nil {
			log.Errorf("Proxy error: %s", err)
		}
		log.Debug("Proxy finished")
	case sig := <-stop:
		log.Infof("Received signal '%s', shutdown application...", sig)
		cancel()
		err := <-proxy.Done() // wait for the proxy to exit
		if err != nil {
			log.Errorf("Error: %s", err)
		}
	}
	log.Info("Tired proxy - exit")
}
