package epoller

import (
	"net"
	"syscall"
)

// Poller is the interface for epoll/kqueue poller, special for network connections.
type Poller interface {
	// Add adds the connection to poller.
	Add(conn net.Conn) error
	// Remove removes the connection from poller and closes it.
	Remove(conn net.Conn) error
	// Wait waits for at most count events and returns the connections.
	Wait(count int) ([]net.Conn, error)
	// Close closes the poller. If closeConns is true, it will close all the connections.
	Close(closeConns bool) error
}

func SocketFD(conn net.Conn) int {
	if con, ok := conn.(syscall.Conn); ok {
		raw, err := con.SyscallConn()
		if err != nil {
			return 0
		}
		sfd := 0
		raw.Control(func(fd uintptr) { // nolint: errcheck
			sfd = int(fd)
		})
		return sfd
	} else if con, ok := conn.(ConnImpl); ok {
		return con.fd
	}
	return 0
}
