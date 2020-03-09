// +build linux darwin netbsd freebsd openbsd dragonfly

package evnio

import (
	"runtime"
	"syscall"

	"github.com/dreamans/evnio/util"

	"github.com/dreamans/evnio/evlog"
)

type server struct {
	addr          string
	protocol      Protocol
	handler       ConnectionHandler
	numLoops      int
	ln            *Listener
	evLoop        *EventLoop
	workEvLoops   []*EventLoop
	nextLoopIndex int
	inShutdown    util.AtomicBool
}

func NewServer(opt *Options) Server {
	return &server{
		addr:     opt.Addr,
		protocol: opt.Protocol,
		handler:  opt.Handler,
		numLoops: opt.NumLoops,
	}
}

func (srv *server) Start() error {
	if srv.inShutdown.IsSet() {
		return ErrServerClosed
	}
	if err := srv.initEventLoop(); err != nil {
		return err
	}
	if err := srv.initListener(srv.addr); err != nil {
		return err
	}
	for i := 0; i < len(srv.workEvLoops); i++ {
		go func(i int) {
			srv.workEvLoops[i].Wait()
		}(i)
	}
	srv.evLoop.Wait()

	return nil
}

func (srv *server) Shutdown() error {
	if srv.inShutdown.IsSet() {
		return ErrServerClosed
	}
	srv.inShutdown.Set()

	for _, loop := range srv.workEvLoops {
		_ = loop.Stop()
	}
	_ = srv.evLoop.Stop()

	return nil
}

func (srv *server) initEventLoop() error {
	evLoop, err := newEventLoop()
	if err != nil {
		return err
	}
	srv.evLoop = evLoop

	if srv.numLoops == 0 {
		srv.numLoops = runtime.NumCPU()
	}

	workEvLoops := make([]*EventLoop, srv.numLoops)
	for i := 0; i < srv.numLoops; i++ {
		loop, err := newEventLoop()
		if err != nil {
			return err
		}
		workEvLoops[i] = loop
	}

	srv.workEvLoops = workEvLoops

	return nil
}

func (srv *server) initListener(addr string) error {
	l, err := NewListener(addr, srv.evLoop, srv.newConnHandler)
	if err != nil {
		return err
	}
	srv.ln = l
	if err := srv.evLoop.AddFdHandler(srv.ln.Fd(), srv.ln); err != nil {
		return err
	}
	return nil
}

func (srv *server) newConnHandler(ncfd int, sa syscall.Sockaddr) {
	workLoop := srv.evLoopBalance()
	c := newConnection(ncfd, workLoop, util.SockAddrToAddr(sa), srv.ln.ln.Addr(), srv.protocol, srv.handler)
	if err := workLoop.AddFdHandler(ncfd, c); err != nil {
		evlog.Error("[workLoop.AddFdHandler]: %s", err.Error())
	}
}

func (srv *server) evLoopBalance() *EventLoop {
	loop := srv.workEvLoops[srv.nextLoopIndex]
	srv.nextLoopIndex = (srv.nextLoopIndex + 1) % len(srv.workEvLoops)
	return loop
}
