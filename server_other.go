// +build !linux,!darwin,!netbsd,!freebsd,!openbsd,!dragonfly

package evnio

import (
	"errors"
	"net"
	"sync"

	"github.com/dreamans/evnio/util"

	"github.com/dreamans/evnio/evlog"
)

type server struct {
	mu         sync.Mutex
	addr       string
	protocol   Protocol
	handler    ConnectionHandler
	ln         *net.TCPListener
	inShutdown util.AtomicBool
}

func NewServer(opt *Options) Server {
	srv := &server{
		addr:     opt.Addr,
		protocol: opt.Protocol,
		handler:  opt.Handler,
	}

	return srv
}

func (srv *server) Start() error {
	if srv.inShutdown.IsSet() {
		return ErrServerClosed
	}

	network, addr := util.ParseListenerAddr(srv.addr)
	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	l, ok := ln.(*net.TCPListener)
	if !ok {
		return errors.New("could not get file descriptor")
	}
	srv.ln = l

	return srv.serve()
}

func (srv *server) Shutdown() error {
	srv.inShutdown.Set()

	return srv.ln.Close()
}

func (srv *server) serve() error {
	for {
		rw, err := srv.ln.AcceptTCP()
		if err != nil {
			if srv.inShutdown.IsSet() {
				return nil
			}
			if !util.TemporaryErr(err) {
				return err
			}
			evlog.Errorf("[srv.ln.Accept]: %s", err.Error())
			continue
		}
		srv.newConnection(rw)
	}
}

func (srv *server) newConnection(rw net.Conn) {
	newConnection(rw, srv.protocol, srv.handler)
}
