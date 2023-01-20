package main

//
// Original code from https://github.com/bitnami/wait-for-port
// Minor adaptions done to make it easier to integrate into tired-proxy.
// Did not import the original code because it was not a lib

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"time"
)

type PortStatus string

const (
	// PortInUse defines the state of a port in use
	PortInUse PortStatus = "inuse"
	// PortFree defines the state of a free port
	PortFree PortStatus = "free"
)

// WaitForPortCmd allows checking a port state
type WaitForPortCmd struct {
	Host    string
	State   PortStatus
	Timeout int
	Port    int
}

// NewWaitForPortCmd returns a WaitForPortCmd with given parameters
func NewWaitForPortCmd(remote *url.URL, portStatus PortStatus, timeout int) *WaitForPortCmd {
	host, port, err := net.SplitHostPort(remote.Host)
	if err != nil {
		// no port in the host, guess for a port
		switch remote.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			panic(fmt.Sprintf("unable to determine port for url %#v, please specify one explicitly", remote.String()))
		}
		host = remote.Host
	}
	portNr, err := strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("unexpected non numeric port discovered %#v", port))
	}
	return &WaitForPortCmd{
		State:   portStatus,
		Host:    host,
		Timeout: timeout,
		Port:    portNr,
	}
}

// Execute performs the port check
func (c *WaitForPortCmd) Wait() error {
	var checkPortState func(ctx context.Context, host string, port int) bool
	switch c.State {
	case PortInUse:
		checkPortState = portIsInUse
	case PortFree:
		checkPortState = func(ctx context.Context, host string, port int) bool {
			return !portIsInUse(ctx, host, port)
		}
	default:
		return fmt.Errorf("unknown state %q", c.State)
	}
	if err := validatePort(c.Port); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Timeout)*time.Second)
	defer cancel()
	if err := validateHost(ctx, c.Host); err != nil {
		return err
	}

	for !checkPortState(ctx, c.Host, c.Port) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout reached before the port went into state %q", c.State)
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil
}

func validatePort(port int) error {
	if port <= 0 {
		return fmt.Errorf("port out of range: port must be greater than zero")
	} else if port > 65535 {
		return fmt.Errorf("port out of range: port must be <= 65535")
	}
	return nil
}

func validateHost(ctx context.Context, host string) error {
	// An empty host is perfectly fine for us but net.LookupHost will fail
	if host == "" {
		return nil
	}
	if _, err := net.DefaultResolver.LookupHost(ctx, host); err != nil {
		return fmt.Errorf("cannot resolve host %q: %v", host, err)
	}
	return nil
}

func isAddrInUseError(err error) bool {
	if err, ok := err.(*net.OpError); ok {
		if err, ok := err.Err.(*os.SyscallError); ok {
			return err.Err == syscall.EADDRINUSE
		}
	}
	return false
}

func canConnectToPort(ctx context.Context, host string, port int) bool {
	d := net.Dialer{Timeout: 60 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err == nil {
		defer conn.Close()
		return true
	}
	return false
}

// portIsInUse allows checking if a port is in use in the specified host.
func portIsInUse(ctx context.Context, host string, port int) bool {
	// If we can connect, is in use
	if canConnectToPort(ctx, host, port) {
		return true
	}

	// If we are trying to check a remote host, we cannot do more, so we consider it not in use
	if host != "" {
		return false
	}

	// If we are checking locally, try to listen
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err == nil {
		listener.Close()
		return false
	} else if isAddrInUseError(err) {
		return true
	}
	// We could not connect to the port, and we cannot listen on it, the safest thing
	// we can assume in localhost is that is not in use (binding to a privileged port, for example)
	return false
}
