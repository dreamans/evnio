// +build linux darwin netbsd freebsd openbsd dragonfly

package evnio

import (
	"errors"
	"net"
	"syscall"

	"github.com/dreamans/evnio/util"

	"github.com/dreamans/evnio/evlog"
	"github.com/dreamans/evnio/poller"
)

type ListenHandler func(ncfd int, sa syscall.Sockaddr)

type Listener struct {
	ln             net.Listener
	fd             int
	newConnHandler ListenHandler
	evLoop         *EventLoop
}

func NewListener(addr string, evLoop *EventLoop, handler ListenHandler) (*Listener, error) {
	listener := &Listener{
		newConnHandler: handler,
		evLoop:         evLoop,
	}

	network, addr := util.ParseListenerAddr(addr)
	ln, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	listener.ln = ln

	fd, err := listener.getNonblockFd()
	if err != nil {
		return nil, err
	}
	listener.fd = fd

	return listener, nil
}

func (l *Listener) Fd() int {
	return l.fd
}

func (l *Listener) EventHandler(fd int, events poller.Event) {
	if events&poller.EventRead != 0 {
		ncfd, sa, err := syscall.Accept(fd)
		if err != nil {
			if err != syscall.EAGAIN {
				evlog.Errorf("[syscall.Accept]: %s", err.Error())
			}
			return
		}
		if err := syscall.SetNonblock(ncfd, true); err != nil {
			_ = syscall.Close(ncfd)
			evlog.Errorf("[syscall.SetNonblock]: %s", err.Error())
			return
		}

		l.callNewConnHandler(ncfd, sa)
	}
}

func (l *Listener) Close() error {
	l.evLoop.Trigger(func() {
		_ = l.evLoop.DelFdHandler(l.fd)
		_ = l.ln.Close()
	})
	return nil
}

func (l *Listener) callNewConnHandler(ncfd int, sa syscall.Sockaddr) {
	if l.newConnHandler != nil {
		l.newConnHandler(ncfd, sa)
	}
}

func (l *Listener) getNonblockFd() (int, error) {
	tcpln, ok := l.ln.(*net.TCPListener)
	if !ok {
		return 0, errors.New("could not get file descriptor")
	}
	file, err := tcpln.File()
	if err != nil {
		return 0, err
	}
	fd := int(file.Fd())
	if err = syscall.SetNonblock(fd, true); err != nil {
		return 0, err
	}

	return fd, nil
}
