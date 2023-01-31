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

func StartProxy(ctx context.Context, proxyTarget *url.URL, port string, idleTimeout time.Duration) *IdleProxy {
	//	ctx, cancel := context.WithCancel(ctx)

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
		log.Info("start proxy server")
		if err := idleProxy.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			idleProxy.chanFinish <- fmt.Errorf("error during handling proxy request: %w", err)
		}
		// Proxy stopped
		log.Info("proxy server shut down")
		close(idleProxy.chanFinish)
	}()

	return idleProxy
}

// Request handler for the idle proxy
func (p *IdleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.timer.Reset(p.idleTimeout)
	//TODO: log.Println(r.URL)
	log.Info(r.URL)
	p.proxy.ServeHTTP(w, r)
}

func (p *IdleProxy) TimerDone() <-chan time.Time {
	return p.timer.C
}

func (p *IdleProxy) Done() <-chan error {
	return p.chanFinish
}
