// +build linux darwin netbsd freebsd openbsd dragonfly

package evnio

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"syscall"

	"github.com/dreamans/evnio/util"

	"github.com/dreamans/evnio/evlog"
	"github.com/dreamans/evnio/poller"
)

type conn struct {
	fd         int
	evLoop     *EventLoop
	handler    ConnectionHandler
	writeBuf   *bytes.Buffer
	readBuf    *bytes.Buffer
	protocol   Protocol
	closed     util.AtomicBool
	localAddr  net.Addr
	remoteAddr net.Addr
	ctx        context.Context
	action     Action
}

func newConnection(fd int, evLoop *EventLoop, caddr net.Addr, saddr net.Addr, pcol Protocol, handler ConnectionHandler) *conn {
	c := &conn{
		fd:         fd,
		evLoop:     evLoop,
		writeBuf:   connBufferPool.Get().(*bytes.Buffer),
		readBuf:    connBufferPool.Get().(*bytes.Buffer),
		remoteAddr: caddr,
		localAddr:  saddr,
		protocol:   pcol,
		handler:    handler,
		action:     ActionNone,
	}
	if pcol == nil {
		c.protocol = &defaultProtocol{}
	}
	if handler == nil {
		c.handler = &defaultConnectionHandler{}
	}
	c.ctx = context.WithValue(context.Background(), ConnectFdContextKey, fd)

	c.writeBuf.Reset()
	c.readBuf.Reset()
	c.handler.OnOpen(c)

	evlog.Debugf("[NewConnection]: loc %s <--> remote %s", c.LocalAddr(), c.RemoteAddr())
	return c
}

func (c *conn) UniqID() uint64 {
	return uint64(c.fd)
}

func (c *conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *conn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *conn) Context() context.Context {
	return c.ctx
}

func (c *conn) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *conn) Send(buffer []byte, action Action) error {
	if c.closed.IsSet() {
		return ErrConnectionClosed
	}
	if len(buffer) == 0 {
		return nil
	}
	c.evLoop.Trigger(func() {
		if !c.closed.IsSet() {
			c.writeBuf.Write(c.protocol.Packet(c, buffer))
			c.action = action
			if err := c.evLoop.EnableReadWrite(c.fd); err != nil {
				evlog.Errorf("[evLoop.EnableReadWrite]: %s", err.Error())
			}
		}
	})
	return nil
}

func (c *conn) Close() error {
	if c.closed.IsSet() {
		return ErrConnectionClosed
	}

	c.evLoop.Trigger(func() {
		c.handleClose(c.fd)
	})
	return nil
}

func (c *conn) EventHandler(fd int, events poller.Event) {
	if events&poller.EventErr != 0 {
		c.handleClose(fd)
		return
	}
	if events&poller.EventRead != 0 {
		c.handleRead(fd)
	}
	if events&poller.EventWrite != 0 {
		c.handleWrite(fd)
	}
}

func (c *conn) handleClose(fd int) {
	if !c.closed.IsSet() {
		c.closed.Set()

		if err := c.evLoop.DelFdHandler(fd); err != nil {
			evlog.Errorf("[evLoop.DelFdHandler]: %s", err.Error())
		}

		c.handler.OnClose(c)
		if err := syscall.Close(fd); err != nil {
			evlog.Errorf("[syscall.Close]: %s", err.Error())
		}

		connBufferPool.Put(c.readBuf)
		connBufferPool.Put(c.writeBuf)

		evlog.Debugf("[HandleClose]: loc %s <-x-> remote %s", c.LocalAddr(), c.RemoteAddr())
	}
}

func (c *conn) handleRead(fd int) {
	buf := c.evLoop.PacketBuf()
	n, err := syscall.Read(fd, buf)
	if n == 0 || err != nil {
		if err != syscall.EAGAIN {
			c.handleClose(fd)
		}
		if err != nil {
			evlog.Errorf("[syscall.Read]: %s", err.Error())
		}
		return
	}

	evlog.Debugf("[HandleRead]: loc %s <- remote %s, len {%d}", c.LocalAddr(), c.RemoteAddr(), n)

	c.readBuf.Write(buf[:n])
	c.protocolUnPacket(c.readBuf)
}

func (c *conn) handleWrite(fd int) {
	if c.writeBuf.Len() > 0 {
		c.writeTo(fd)
	} else if c.action != ActionNone {
		c.actionTo(fd)
	}
	if c.writeBuf.Len() == 0 && c.action == ActionNone {
		if err := c.evLoop.EnableRead(c.fd); err != nil {
			evlog.Errorf("[evLoop.EnableRead]: %s", err.Error())
		}
	}
}

func (c *conn) writeTo(fd int) {
	n, err := syscall.Write(fd, c.writeBuf.Bytes())
	if err != nil {
		if err == syscall.EAGAIN {
			return
		}
		// write failed, remove EVFILT_WRITE
		_ = c.evLoop.EnableRead(c.fd)

		c.handleClose(fd)
		evlog.Errorf("[syscall.Write]: %s", err.Error())
		return
	}

	evlog.Debugf("[HandleWrite]: loc %s -> remote %s, len {%d}", c.LocalAddr(), c.RemoteAddr(), n)

	if n == c.writeBuf.Len() {
		c.writeBuf.Reset()
	} else {
		c.writeBuf.Next(n)
	}
}

func (c *conn) actionTo(fd int) {
	switch c.action {
	default:
		c.action = ActionNone
	case ActionClose:
		c.handleClose(fd)
	}
	fmt.Println(c.action)
}

func (c *conn) protocolUnPacket(buffer *bytes.Buffer) {
	for {
		data := c.protocol.UnPacket(c, buffer)
		if len(data) == 0 {
			break
		}
		c.handler.OnMessage(c, data)
	}
}
