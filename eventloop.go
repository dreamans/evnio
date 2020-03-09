package evnio

import (
	"sync"

	"github.com/dreamans/evnio/poller"
)

type EventLoop struct {
	mu       sync.Mutex
	poll     poller.Poller
	handlers sync.Map
	packet   []byte
	triggers []func()
}

type EventHandler interface {
	EventHandler(fd int, events poller.Event)
	Close() error
}

func newEventLoop() (*EventLoop, error) {
	evLoop := &EventLoop{
		packet: make([]byte, 0xFFFF),
	}
	poll, err := poller.New(evLoop.eventHandler)
	if err != nil {
		return nil, err
	}
	evLoop.poll = poll

	return evLoop, nil
}

func (ev *EventLoop) Trigger(fn func()) {
	ev.mu.Lock()
	ev.triggers = append(ev.triggers, fn)
	ev.mu.Unlock()

	_ = ev.poll.Trigger()
}

func (ev *EventLoop) PacketBuf() []byte {
	return ev.packet
}

func (ev *EventLoop) AddFdHandler(fd int, handler EventHandler) error {
	if err := ev.poll.AddRead(fd); err != nil {
		return err
	}
	ev.handlers.Store(fd, handler)
	return nil
}

func (ev *EventLoop) DelFdHandler(fd int) error {
	ev.handlers.Delete(fd)
	if err := ev.poll.Del(fd); err != nil {
		return err
	}
	return nil
}

func (ev *EventLoop) EnableReadWrite(fd int) error {
	return ev.poll.EnableReadWrite(fd)
}

func (ev *EventLoop) EnableRead(fd int) error {
	return ev.poll.EnableRead(fd)
}

func (ev *EventLoop) Wait() {
	ev.poll.Wait()
}

func (ev *EventLoop) Stop() error {
	ev.handlers.Range(func(key, value interface{}) bool {
		if f, ok := value.(EventHandler); ok {
			_ = f.Close()
		}
		return true
	})
	return ev.poll.Close()
}

func (ev *EventLoop) eventHandler(fd int, events poller.Event) {
	if fd > 0 {
		handler, ok := ev.handlers.Load(fd)
		if ok {
			handler.(EventHandler).EventHandler(fd, events)
		}
	}

	ev.doTriggers()
}

func (ev *EventLoop) doTriggers() {
	ev.mu.Lock()
	fns := ev.triggers
	ev.triggers = nil
	ev.mu.Unlock()

	for _, fn := range fns {
		fn()
	}
}
