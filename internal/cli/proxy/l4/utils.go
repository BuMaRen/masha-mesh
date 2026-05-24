package l4

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func isHttp(conn *bufio.Reader) bool {
	peek, err := conn.Peek(64)
	if err != nil {
		return false
	}
	return bytes.HasPrefix(peek, []byte("GET ")) ||
		bytes.HasPrefix(peek, []byte("POST ")) ||
		bytes.HasPrefix(peek, []byte("HEAD ")) ||
		bytes.HasPrefix(peek, []byte("PUT ")) ||
		bytes.HasPrefix(peek, []byte("DELETE ")) ||
		bytes.HasPrefix(peek, []byte("OPTIONS ")) ||
		bytes.HasPrefix(peek, []byte("CONNECT ")) ||
		bytes.HasPrefix(peek, []byte("TRACE ")) ||
		bytes.HasPrefix(peek, []byte("HTTP/1. "))
}

func getOriginalDst(conn net.Conn) (*net.TCPAddr, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, fmt.Errorf("not a TCP connection")
	}
	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("getting syscall.RawConn: %w", err)
	}
	originalDst := &net.TCPAddr{}
	var sockerr error
	rawConn.Control(func(fd uintptr) {
		addr, err := syscall.GetsockoptIPv6Mreq(int(fd), unix.IPPROTO_IP, unix.SO_ORIGINAL_DST)
		if err != nil {
			sockerr = err
			return
		}
		ip := net.IPv4(addr.Multiaddr[4], addr.Multiaddr[5], addr.Multiaddr[6], addr.Multiaddr[7])
		port := int(addr.Multiaddr[2])<<8 + int(addr.Multiaddr[3])
		originalDst = &net.TCPAddr{IP: ip, Port: port}
	})
	return originalDst, sockerr
}
