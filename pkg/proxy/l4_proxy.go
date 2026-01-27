package proxy

import (
	"io"
	"log"
	"net"
	"sync"
)

// L4Proxy implements a TCP Layer 4 proxy
type L4Proxy struct {
	listenAddr string
	targetAddr string
}

// NewL4Proxy creates a new L4 proxy
func NewL4Proxy(listenAddr, targetAddr string) *L4Proxy {
	return &L4Proxy{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
	}
}

// Start starts the L4 proxy server
func (p *L4Proxy) Start() error {
	listener, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	
	log.Printf("L4 Proxy listening on %s, forwarding to %s", p.listenAddr, p.targetAddr)
	
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		
		go p.handleConnection(clientConn)
	}
}

func (p *L4Proxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()
	
	// Connect to target
	targetConn, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		log.Printf("Error connecting to target %s: %v", p.targetAddr, err)
		return
	}
	defer targetConn.Close()
	
	log.Printf("L4 Proxy: New connection from %s to %s", clientConn.RemoteAddr(), p.targetAddr)
	
	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)
	
	// Client -> Target
	go func() {
		defer wg.Done()
		io.Copy(targetConn, clientConn)
		targetConn.(*net.TCPConn).CloseWrite()
	}()
	
	// Target -> Client
	go func() {
		defer wg.Done()
		io.Copy(clientConn, targetConn)
		clientConn.(*net.TCPConn).CloseWrite()
	}()
	
	wg.Wait()
	log.Printf("L4 Proxy: Connection closed from %s", clientConn.RemoteAddr())
}
