package l4

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/BuMaRen/mesh/pkg/utils"
	"k8s.io/klog/v2"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run(ctx context.Context, opts *Options) error {
	listener := utils.NewListenerOrDie("tcp", opts.Address())
	defer listener.Close()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, err := accept(listener)
			if err != nil {
				klog.Errorf("accept connection failed with error: %+v", err)
				continue
			}
			if isHttp(conn.Reader) {
				// Handle HTTP traffic with L7 proxy
				go transferToL7(ctx, conn, opts.DstL7Address())
				continue
			}
			// 不是 HTTP 流量，进行 TCP 透传
			go transmission(ctx, conn.Connection())
		}
	}
}

func transmission(ctx context.Context, conn net.Conn) {
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
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done: // 正常结束
			conn.Close()
			targetConn.Close()
		case <-ctx.Done():
			conn.Close()
			targetConn.Close()
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(targetConn, conn); err != nil {
			klog.Errorf("copy from conn to targetConn failed with error: %+v", err)
		}
		if tcpConn, ok := targetConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err = io.Copy(conn, targetConn); err != nil {
			klog.Errorf("copy from targetConn to conn failed with error: %+v", err)
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	wg.Wait()
}

func transferToL7(ctx context.Context, conn *IoWR, l7Address string) {
	output, err := net.DialTimeout("tcp", l7Address, 5*time.Second)
	if err != nil {
		klog.Errorf("dialing to http server: %v", err)
		conn.Close()
		return
	}
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done: // 正常结束
			output.Close()
			conn.Close()
		case <-ctx.Done():
			output.Close()
			conn.Close()
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(output, conn); err != nil {
			klog.Errorf("copy from conn to output failed with error: %+v", err)
		}
		if tcpConn, ok := output.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn.Connection(), output); err != nil {
			klog.Errorf("copy from output to conn failed with error: %+v", err)
		}
		if tcpConn, ok := conn.Connection().(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	wg.Wait()
}
