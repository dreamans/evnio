// +build linux

package poller

import (
	"syscall"
	"time"

	"github.com/dreamans/evnio/evlog"
	"github.com/dreamans/evnio/util"
)

const (
	readEvent  = syscall.EPOLLIN | syscall.EPOLLPRI
	writeEvent = syscall.EPOLLOUT
)

var (
	wakeWriteBytes = []byte{1, 0, 0, 0, 0, 0, 0, 0}
	wakeBuf        = make([]byte, 8)
)

type Epoll struct {
	fd        int
	eventFd   int
	handler   EventHandler
	closed    util.AtomicBool
	closeDone chan struct{}
}

func New(handler EventHandler) (Poller, error) {
	return EpollCreate(handler)
}

func EpollCreate(handler EventHandler) (*Epoll, error) {
	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	r0, _, e0 := syscall.Syscall(syscall.SYS_EVENTFD2, 0, 0, 0)
	if e0 != 0 {
		return nil, e0
	}
	epoller := &Epoll{
		fd:        fd,
		handler:   handler,
		eventFd:   int(r0),
		closeDone: make(chan struct{}),
	}

	if err := epoller.AddRead(epoller.eventFd); err != nil {
		_ = syscall.Close(fd)
		_ = syscall.Close(epoller.eventFd)
		return nil, err
	}

	return epoller, nil
}

func (ep *Epoll) Trigger() error {
	_, err := syscall.Write(ep.eventFd, wakeWriteBytes)
	return err
}

func (ep *Epoll) triggerHandlerRead() {
	_, _ = syscall.Read(ep.eventFd, wakeBuf)
}

func (ep *Epoll) AddRead(fd int) error {
	return ep.add(fd, readEvent)
}

func (ep *Epoll) EnableReadWrite(fd int) error {
	return ep.mod(fd, readEvent|writeEvent)
}

func (ep *Epoll) EnableRead(fd int) error {
	return ep.mod(fd, readEvent)
}

func (ep *Epoll) Del(fd int) error {
	return syscall.EpollCtl(ep.fd, syscall.EPOLL_CTL_DEL, fd, nil)
}

func (ep *Epoll) Close() error {
	if ep.closed.IsSet() {
		return ErrClosed
	}
	ep.closed.Set()
	_ = ep.Trigger()

	<-ep.closeDone

	return syscall.Close(ep.fd)
}

func (ep *Epoll) Wait() {
	defer func() {
		close(ep.closeDone)
	}()

	events := make([]syscall.EpollEvent, waitEventsBeginNum)
	trigger := false

	var tempDelay time.Duration
	for {
		n, err := syscall.EpollWait(ep.fd, events, -1)

		if err != nil && !util.TemporaryErr(err) {
			if tempDelay == 0 {
				tempDelay = 5 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 500 * time.Millisecond; tempDelay >= max {
				tempDelay = max
			}

			evlog.Errorf("[syscall.EpollWait]: %s", err.Error())
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			if fd != ep.eventFd {
				var event Event
				if events[i].Events&(syscall.EPOLLIN|syscall.EPOLLPRI|syscall.EPOLLRDHUP) != 0 {
					event |= EventRead
				}
				if (events[i].Events&syscall.EPOLLERR != 0) || (events[i].Events&syscall.EPOLLOUT != 0) {
					event |= EventWrite
				}
				if ((events[i].Events & syscall.EPOLLHUP) != 0) && ((events[i].Events & syscall.EPOLLIN) == 0) {
					event |= EventErr
				}
				ep.handler(fd, event)
			} else {
				ep.triggerHandlerRead()
				trigger = true
			}
		}
		if trigger {
			ep.handler(-1, 0)
			if ep.closed.IsSet() {
				return
			}
			trigger = false
		}
		if n == len(events) {
			events = make([]syscall.EpollEvent, int(float64(n)*1.5))
		}
	}
}

func (ep *Epoll) add(fd int, events Event) error {
	ev := &syscall.EpollEvent{
		Fd:     int32(fd),
		Events: uint32(events),
	}
	return syscall.EpollCtl(ep.fd, syscall.EPOLL_CTL_ADD, fd, ev)
}

func (ep *Epoll) mod(fd int, events Event) error {
	ev := &syscall.EpollEvent{
		Events: uint32(events),
		Fd:     int32(fd),
	}
	return syscall.EpollCtl(ep.fd, syscall.EPOLL_CTL_MOD, fd, ev)
}
