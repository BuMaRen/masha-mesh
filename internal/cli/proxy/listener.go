package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"syscall"
	"time"

	"github.com/BuMaRen/mesh/internal/cli/breaker"
	"github.com/BuMaRen/mesh/internal/cli/proxyconfig"
	"github.com/BuMaRen/mesh/internal/cli/rpcclient"
	"github.com/BuMaRen/mesh/pkg/utils"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

type Listener struct {
	meshClient *rpcclient.MeshClient
	config     *proxyconfig.Config
	breaker    *breaker.Breaker
}

func NewListener(meshClient *rpcclient.MeshClient, config *proxyconfig.Config, brk *breaker.Breaker) *Listener {
	return &Listener{
		meshClient: meshClient,
		config:     config,
		breaker:    brk,
	}
}

func (l *Listener) Listen(ctx context.Context, address string) error {
	listener := utils.NewListenerOrDie("tcp", address)
	defer listener.Close()

	// Ensure Accept unblocks promptly on context cancellation.
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	klog.Infof("[Proxy] listening on %s", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			klog.Errorf("[Proxy] accept connection failed: %+v", err)
			continue
		}
		go l.handleConnection(ctx, conn)
	}
}

func (l *Listener) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	if isHTTP(reader) {
		l.handleHTTP(ctx, conn, reader)
	} else {
		l.handleTCP(ctx, conn, reader)
	}
}

func (l *Listener) handleHTTP(_ context.Context, conn net.Conn, reader *bufio.Reader) {
	klog.V(4).Info("[Proxy] handling HTTP connection")

	proxy := &httputil.ReverseProxy{
		Transport: &httpTransport{
			listener: l,
		},
		Director: func(req *http.Request) {},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			klog.Errorf("[Proxy] HTTP proxy error: %v", err)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	wrappedConn := &connWrapper{
		reader: reader,
		Conn:   conn,
	}

	httpConn := newHTTPConn(wrappedConn)
	defer httpConn.Close()

	httpConn.Serve(proxy)
}

func (l *Listener) handleTCP(ctx context.Context, conn net.Conn, reader *bufio.Reader) {
	klog.V(4).Info("[Proxy] handling TCP connection")

	originalDst, err := getOriginalDst(conn)
	if err != nil {
		klog.Errorf("[Proxy] get original destination failed: %+v", err)
		return
	}

	targetConn, err := net.Dial("tcp", originalDst.String())
	if err != nil {
		klog.Errorf("[Proxy] dial to target %s failed: %+v", originalDst.String(), err)
		return
	}
	defer targetConn.Close()

	wrappedConn := &connWrapper{
		reader: reader,
		Conn:   conn,
	}

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done:
			conn.Close()
			targetConn.Close()
		case <-ctx.Done():
			conn.Close()
			targetConn.Close()
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(targetConn, wrappedConn); err != nil {
			klog.V(4).Infof("[Proxy] copy from client to target failed: %+v", err)
		}
		if tcpConn, ok := targetConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, targetConn); err != nil {
			klog.V(4).Infof("copy from target to client failed: %+v", err)
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
}

// peek 查看是否是 HTTP 请求
func isHTTP(reader *bufio.Reader) bool {
	peek, err := reader.Peek(64)
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
		bytes.HasPrefix(peek, []byte("PATCH ")) ||
		bytes.HasPrefix(peek, []byte("HTTP/1."))
}

// 获取原始目的地址
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
	if err := rawConn.Control(func(fd uintptr) {
		addr, err := syscall.GetsockoptIPv6Mreq(int(fd), unix.IPPROTO_IP, unix.SO_ORIGINAL_DST)
		if err != nil {
			sockerr = err
			return
		}
		ip := net.IPv4(addr.Multiaddr[4], addr.Multiaddr[5], addr.Multiaddr[6], addr.Multiaddr[7])
		port := int(addr.Multiaddr[2])<<8 + int(addr.Multiaddr[3])
		originalDst = &net.TCPAddr{IP: ip, Port: port}
	}); err != nil {
		return nil, fmt.Errorf("rawConn.Control: %w", err)
	}
	if sockerr != nil {
		return nil, sockerr
	}
	return originalDst, nil
}

type connWrapper struct {
	reader *bufio.Reader
	net.Conn
}

func (c *connWrapper) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

// 处理单条 HTTP 连接，供 http.Server 使用
type httpConn struct {
	conn net.Conn
}

func newHTTPConn(conn net.Conn) *httpConn {
	return &httpConn{conn: conn}
}

func (h *httpConn) Serve(handler http.Handler) {
	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	listener := &singleConnListener{
		conn: h.conn,
		done: make(chan struct{}),
	}

	server.Serve(listener)
}

func (h *httpConn) Close() error {
	return h.conn.Close()
}

// 实现 net.Listener 接口，一次性连接，供 http.Server 使用
type singleConnListener struct {
	conn   net.Conn
	done   chan struct{}
	once   sync.Once
	closed bool
	mu     sync.Mutex
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, net.ErrClosed
	}

	conn := l.conn
	l.closed = true
	l.once.Do(func() { close(l.done) })

	if conn == nil {
		return nil, net.ErrClosed
	}

	return conn, nil
}

func (l *singleConnListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.closed {
		l.closed = true
		l.once.Do(func() { close(l.done) })
	}
	return nil
}

func (l *singleConnListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}
