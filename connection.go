package evnio

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync"
)

var ErrConnectionClosed = errors.New("connection closed")

const (
	ConnectFdContextKey = "connect-fd-context-key"
)

type Connection interface {
	UniqID() uint64

	RemoteAddr() net.Addr

	LocalAddr() net.Addr

	Context() context.Context

	SetContext(context.Context)

	Send([]byte, Action) error

	Close() error
}

type ConnectionHandler interface {
	OnOpen(c Connection)
	OnMessage(c Connection, data []byte)
	OnClose(c Connection)
}

type defaultConnectionHandler struct{}

func (*defaultConnectionHandler) OnOpen(c Connection)                 {}
func (*defaultConnectionHandler) OnMessage(c Connection, data []byte) {}
func (*defaultConnectionHandler) OnClose(c Connection)                {}

var connBufferPool = NewBufferPoll()

func NewBufferPoll() (pool sync.Pool) {
	pool.New = func() interface{} {
		return &bytes.Buffer{}
	}
	return pool
}
