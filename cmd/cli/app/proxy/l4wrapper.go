package proxy

import (
	"bufio"
	"net"
)

type IoWR struct {
	*bufio.Reader
	*bufio.Writer
	handle net.Conn
}

func newWR(fromApp net.Conn) *IoWR {
	return &IoWR{
		Reader: bufio.NewReader(fromApp),
		Writer: bufio.NewWriter(fromApp),
		handle: fromApp,
	}
}

func (w *IoWR) Connection() net.Conn {
	return w.handle
}

func (w *IoWR) Close() error {
	return w.handle.Close()
}

func accept(listener net.Listener) (*IoWR, error) {
	conn, err := listener.Accept()
	if err != nil {
		return nil, err
	}
	return newWR(conn), nil
}
