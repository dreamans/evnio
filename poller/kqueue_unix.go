// +build darwin netbsd freebsd openbsd dragonfly

package poller

import (
	"syscall"
	"time"

	"github.com/dreamans/evnio/evlog"
	"github.com/dreamans/evnio/util"
)

type KQueue struct {
	handler   EventHandler
	fd        int
	closed    util.AtomicBool
	closeDone chan struct{}
}

func New(handler EventHandler) (Poller, error) {
	return KQueueCreate(handler)
}

func KQueueCreate(handler EventHandler) (*KQueue, error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}

	_, err = syscall.Kevent(fd, []syscall.Kevent_t{{
		Ident:  0,
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)
	if err != nil {
		return nil, err
	}

	kq := &KQueue{
		handler:   handler,
		fd:        fd,
		closeDone: make(chan struct{}),
	}
	return kq, nil
}

func (kq *KQueue) Trigger() error {
	_, err := syscall.Kevent(kq.fd, []syscall.Kevent_t{{
		Ident:  0,
		Filter: syscall.EVFILT_USER,
		Fflags: syscall.NOTE_TRIGGER,
	}}, nil, nil)
	return err
}

func (kq *KQueue) AddRead(fd int) error {
	_, err := syscall.Kevent(kq.fd, []syscall.Kevent_t{
		{Ident: uint64(fd), Flags: syscall.EV_ADD, Filter: syscall.EVFILT_READ},
	}, nil, nil)
	return err
}

func (kq *KQueue) EnableReadWrite(fd int) error {
	_, err := syscall.Kevent(kq.fd, []syscall.Kevent_t{
		{Ident: uint64(fd), Flags: syscall.EV_ADD, Filter: syscall.EVFILT_WRITE},
	}, nil, nil)
	return err
}

func (kq *KQueue) EnableRead(fd int) error {
	_, err := syscall.Kevent(kq.fd, []syscall.Kevent_t{
		{Ident: uint64(fd), Flags: syscall.EV_DELETE, Filter: syscall.EVFILT_WRITE},
	}, nil, nil)
	return err
}

func (kq *KQueue) Del(fd int) error {
	_, err := syscall.Kevent(kq.fd, []syscall.Kevent_t{
		{Ident: uint64(fd), Flags: syscall.EV_DELETE, Filter: syscall.EVFILT_READ},
	}, nil, nil)
	return err
}

func (kq *KQueue) Wait() {
	defer func() {
		close(kq.closeDone)
	}()

	events := make([]syscall.Kevent_t, waitEventsBeginNum)
	trigger := false

	var tempDelay time.Duration
	for {
		n, err := syscall.Kevent(kq.fd, nil, events, nil)

		if err != nil && !util.TemporaryErr(err) {
			if tempDelay == 0 {
				tempDelay = 5 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 500 * time.Millisecond; tempDelay >= max {
				tempDelay = max
			}

			evlog.Errorf("[syscall.Kevent]: %s", err.Error())
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0

		for i := 0; i < n; i++ {
			fd := int(events[i].Ident)
			if fd != 0 {
				var event Event
				if (events[i].Flags&syscall.EV_ERROR != 0) || (events[i].Flags&syscall.EV_EOF != 0) {
					event |= EventErr
				}
				if events[i].Filter == syscall.EVFILT_WRITE {
					event |= EventWrite
				}
				if events[i].Filter == syscall.EVFILT_READ {
					event |= EventRead
				}
				kq.handler(fd, event)
			} else {
				trigger = true
			}
		}
		if trigger {
			kq.handler(-1, 0)
			if kq.closed.IsSet() {
				return
			}
			trigger = false
		}
		if n == len(events) {
			events = make([]syscall.Kevent_t, int(float64(n)*1.5))
		}
	}
}

func (kq *KQueue) Close() (err error) {
	if kq.closed.IsSet() {
		return ErrClosed
	}
	kq.closed.Set()
	_ = kq.Trigger()

	<-kq.closeDone

	return syscall.Close(kq.fd)
}
