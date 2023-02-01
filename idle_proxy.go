package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type IdleProxy struct {
	idleTimeout time.Duration
	timer       *time.Timer
	server      *http.Server
	proxy       *httputil.ReverseProxy
	chanFinish  chan error
}

func StartIdleProxy(ctx context.Context, proxyTarget *url.URL, port string, idleTimeout time.Duration) *IdleProxy {
	log.Infof("Setup proxy server for target %s", proxyTarget)
	idleProxy := &IdleProxy{
		idleTimeout: idleTimeout,
		timer:       time.NewTimer(idleTimeout),
		server:      &http.Server{Addr: fmt.Sprintf(":%s", port)},
		proxy:       httputil.NewSingleHostReverseProxy(proxyTarget),
		chanFinish:  make(chan error, 1),
	}

	idleProxy.server.Handler = idleProxy

	// wait for the idleTimer to expire, or the context to cancel
	go func() {
		select {
		case <-idleProxy.TimerDone():
			log.Infof("Idle time (%s seconds) passed, shutting down proxy...", idleProxy.idleTimeout.String())
		case <-ctx.Done():
			log.Info("Shutting down proxy...")
		}
		ctx, cancelShutdown := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancelShutdown()
		if err := idleProxy.server.Shutdown(ctx); err != nil {
			idleProxy.chanFinish <- fmt.Errorf("error while shutting down proxy server: %w", err)
		}
	}()

	// start proxy
	go func() {
		log.Infof("Start proxy server, serving at http://localhost%s", idleProxy.server.Addr)
		// ignore ErrServerClosed as this one will be fired when the other goroutine shuts down the server
		if err := idleProxy.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			idleProxy.chanFinish <- fmt.Errorf("error during handling proxy request: %w", err)
		}
		// Proxy stopped
		log.Info("Proxy server shut down")
		close(idleProxy.chanFinish)
	}()

	return idleProxy
}

// Proxy request handler that also resets the idle timer
func (p *IdleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.timer.Reset(p.idleTimeout)
	log.Debugf("%s %s", r.Method, r.URL)
	p.proxy.ServeHTTP(w, r)
}

// Returns a channel that returns the current time when the timer expires
func (p *IdleProxy) TimerDone() <-chan time.Time {
	return p.timer.C
}

// Channel that is closed when the proxy server is shut down.
// If any error occurred during start or shut down of the proxy server, it is sent through the channel.
func (p *IdleProxy) Done() <-chan error {
	return p.chanFinish
}
