package poller

import "errors"

type (
	Event uint32

	EventHandler func(fd int, event Event)
)

const (
	EventRead  Event = 0x1
	EventWrite Event = 0x2
	EventErr   Event = 0x4
)

const (
	waitEventsBeginNum = 128
)

var (
	ErrClosed = errors.New("poller is not running")
)

type Poller interface {
	AddRead(fd int) error
	EnableRead(fd int) error
	EnableReadWrite(fd int) error
	Del(fd int) error
	Wait()
	Trigger() error
	Close() error
}
