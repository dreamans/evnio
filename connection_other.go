// +build !linux,!darwin,!netbsd,!freebsd,!openbsd,!dragonfly

package evnio

import (
	"bytes"
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dreamans/evnio/util"

	"github.com/dreamans/evnio/evlog"
)

type conn struct {
	mu         sync.Mutex
	rw         net.Conn
	handler    ConnectionHandler
	protocol   Protocol
	closed     util.AtomicBool
	cancelCtx  context.CancelFunc
	readBuf    *bytes.Buffer
	writeQueue chan []byte
	action     Action
	uniqID     uint64
}

var connUniqueIncr uint64

func newConnection(rw net.Conn, pcol Protocol, handler ConnectionHandler) *conn {
	c := &conn{
		rw:         rw,
		readBuf:    connBufferPool.Get().(*bytes.Buffer),
		writeQueue: make(chan []byte, 16),
		handler:    handler,
		protocol:   pcol,
		action:     ActionNone,
		uniqID:     atomic.AddUint64(&connUniqueIncr, 1),
	}
	if pcol == nil {
		c.protocol = &defaultProtocol{}
	}
	if handler == nil {
		c.handler = &defaultConnectionHandler{}
	}
	c.readBuf.Reset()
	c.handler.OnOpen(c)

	cc, cancelCtx := context.WithCancel(context.Background())
	c.cancelCtx = cancelCtx
	c.accept(cc)

	evlog.Debugf("[NewConnection]: loc %s <--> remote %s", c.LocalAddr(), c.RemoteAddr())
	return c
}

func (c *conn) UniqID() uint64 {
	return c.uniqID
}

func (c *conn) RemoteAddr() net.Addr {
	return c.rw.RemoteAddr()
}

func (c *conn) LocalAddr() net.Addr {
	return c.rw.LocalAddr()
}

func (c *conn) Fd() int {
	file, err := c.rw.(*net.TCPConn).File()
	if err != nil {
		return -1
	}
	return int(file.Fd())
}

func (c *conn) Send(buffer []byte, action Action) error {
	if c.closed.IsSet() {
		return ErrConnectionClosed
	}
	c.writeQueue <- buffer
	return nil
}

func (c *conn) Close() error {
	if c.closed.IsSet() {
		return ErrConnectionClosed
	}
	return c.handleClose()
}

func (c *conn) handleClose() error {
	c.mu.Lock()
	defer c.mu.Lock()

	if !c.closed.IsSet() {
		c.closed.Set()
		c.cancelCtx()
		c.handler.OnClose(c)
		connBufferPool.Put(c.readBuf)

		evlog.Debugf("[HandleClose]: loc %s <-x-> remote %s", c.LocalAddr(), c.RemoteAddr())
		return c.rw.Close()
	}

	return nil
}

func (c *conn) accept(ctx context.Context) {
	readerCtx, _ := context.WithCancel(ctx)
	go c.readAndWait(readerCtx)

	writerCtx, _ := context.WithCancel(ctx)
	go c.writeAndWait(writerCtx)
}

func (c *conn) readAndWait(ctx context.Context) {
	buf := make([]byte, 0x400)
	for {
		n, err := c.rw.Read(buf)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err != nil {
			_ = c.handleClose()
			return
		}

		evlog.Debugf("[HandleRead]: loc %s -> remote %s, len {%d}, data {%v}", c.LocalAddr(), c.RemoteAddr(), n, buf[:n])

		c.readBuf.Write(buf[:n])
		for {
			data := c.protocol.UnPacket(c, c.readBuf)
			if len(data) == 0 {
				break
			}
			c.handler.OnMessage(c, data)
		}
	}
}

func (c *conn) writeAndWait(ctx context.Context) {
	for {
		select {
		case data := <-c.writeQueue:
			packData := c.protocol.Packet(c, data)
			for {
				n, err := c.rw.Write(packData)

				evlog.Debugf("[HandleWrite]: loc %s <- remote %s, len {%d}, data {%v}", c.LocalAddr(), c.RemoteAddr(), n, packData[:n])

				if err != nil {
					_ = c.handleClose()
					return
				}
				if n == len(packData) {
					break
				}
				packData = packData[n:]
			}
		case <-ctx.Done():
			return
		}
	}
}
