package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
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
	var serverCmd *ServerCommand
	signalCh := make(chan os.Signal, 2)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	args, cmdArgs := splitArgs(os.Args[1:])
	fs := flag.NewFlagSet("main options", flag.ContinueOnError)
	var host = fs.String("host", "http://localhost", "host")
	var port = fs.String("port", "8080", "port")
	var timeInSeconds = fs.Int("time", 60, "time in seconds")
	var upstreamTimeout = fs.Int("upstream-timeout", 0, "maximum time to wait before the upstream server is online")
	fs.Parse(args)

	fmt.Println("tired-proxy")

	remote, err := url.Parse(*host)
	if err != nil {
		panic(err)
	}

	// start server if any
	cmdStr := strings.Join(cmdArgs, " ")
	if cmdStr != "" {
		serverCmd, err = StartServerCommand(ctx, cmdStr)
		if err != nil {
			panic(fmt.Errorf("unable to start server command: %w", err))
		}
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
			p.ServeHTTP(w, r)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	http.HandleFunc("/", handler(proxy))

	fmt.Println("Starting server")

	go func() {
		exitServerCmd := func(sig os.Signal) {
			// stop command server, if any
			if cmdStr != "" {
				if err := serverCmd.cmd.Process.Signal(sig); err != nil && !strings.HasSuffix(err.Error(), "process already finished") {
					fmt.Printf("Failed to shutdown server command: %s\n", err)
					return
				}
				fmt.Println("waiting for exec process to close")
				if err := <-serverCmd.exitCh; err != nil && !strings.HasPrefix(err.Error(), "signal:") {
					fmt.Printf("error while waiting for server command to shut down: %s", err)
					return
				}
			}
		}
		// wait for server CMD to end, or a signal ending the program
		exitCode := 0
		select {
		case err := <-serverCmd.exitCh:
			// cancel context
			cancel()
			// See if se can find the exit code
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ProcessState.ExitCode()
				fmt.Printf("server command exited with exit code %d\n", exitCode)
			} else if err != nil {
				exitCode = 1
				fmt.Printf("server commanded exited with an error,: %s\n", err)
			} else {
				fmt.Printf("server command exited successfully\n")
			}
		case <-idle.Done():
			fmt.Println("Idle time passed, shutting down server")
			exitServerCmd(syscall.SIGTERM)
			cancel()
		case sig := <-signalCh:
			fmt.Println("Signal received, shutting down server")
			exitServerCmd(sig)
			cancel()
		}
		fmt.Println("Shutdown tired-proxy")
		os.Exit(exitCode)
	}()

	err = http.ListenAndServe(fmt.Sprintf(":%s", *port), nil)
	if err != nil {
		panic(err)
	}
}

// splitArgs returns the list of args before and after a "--" arg. If the double
// dash is not specified, then args0 is args and args1 is empty.
func splitArgs(args []string) (args0, args1 []string) {
	for i, v := range args {
		if v == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}
