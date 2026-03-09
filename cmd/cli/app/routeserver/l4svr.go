package routeserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

type L4RouteServer struct {
	address string
	l7Port  int
}

func NewL4RouteServer(address string) *L4RouteServer {
	return &L4RouteServer{
		address: address,
	}
}

func (l4 *L4RouteServer) Complete() {
	_, _, err := net.SplitHostPort(l4.address)
	if err != nil {
		panic(err)
	}
}

func (l4 *L4RouteServer) ProxyLoop() {
	listener, err := net.Listen("tcp", l4.address)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		conn, err := accept(listener)
		if err != nil {
			continue
		}
		if isHttp(conn.Reader) {
			// Handle HTTP traffic with L7 proxy
			go l4.transferToL7(conn, l4.l7Port)
			continue
		}
		// 不是 HTTP 流量，进行 TCP 透传
		go l4.transmission(conn.Connection())
	}
}

// TCP 透传, 将流量直接转发到原始目的地, 直到双端关闭
func (l4 *L4RouteServer) transmission(conn net.Conn) {
	originalDst, err := getOriginalDst(conn)
	if err != nil {
		klog.Errorf("get remote ip from conn failed with error: %+v", err)
		return
	}
	targetConn, err := net.Dial("tcp", originalDst.String())
	if err != nil {
		klog.Errorf("dial to target %s failed with error: %+v", originalDst.String(), err)
		return
	}
	defer targetConn.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer conn.Close()
		_, gerr := io.Copy(targetConn, conn)
		if gerr != nil {
			klog.Errorf("copy from conn to targetConn failed with error: %+v", gerr)
		}
	}()
	_, err = io.Copy(conn, targetConn)
	if err != nil {
		klog.Errorf("copy from targetConn to conn failed with error: %+v", err)
	}
	wg.Wait()
}

// 将 HTTP 流量转发到 L7 代理, 直到双端关闭
func (l4 *L4RouteServer) transferToL7(conn *IoWR, l7Port int) {
	output, err := net.DialTimeout("tcp", fmt.Sprintf(":%v", l7Port), 5*time.Second)
	if err != nil {
		klog.Errorf("dialing to http server: %v", err)
		return
	}
	defer output.Close()
	defer conn.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 := io.Copy(output, conn)
		if err1 != nil {
			klog.Errorf("copy from conn to output failed with error: %+v", err1)
		}
	}()
	_, err = io.Copy(conn, output)
	if err != nil {
		klog.Errorf("copy from output to conn failed with error: %+v", err)
	}
	wg.Wait()
}

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
