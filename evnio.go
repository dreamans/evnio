package evnio

import (
	"errors"
)

type Server interface {
	Start() error
	Shutdown() error
}

type Action uint8

const (
	ActionNone Action = iota
	ActionClose
)

var ErrServerClosed = errors.New("evnio: Server closed")

type Options struct {
	Addr     string
	NumLoops int
	Protocol Protocol
	Handler  ConnectionHandler
}

func NewOptions() *Options {
	return &Options{}
}

func (opts *Options) SetAddr(addr string) *Options {
	opts.Addr = addr
	return opts
}

func (opts *Options) SetNumLoops(num int) *Options {
	opts.NumLoops = num
	return opts
}

func (opts *Options) SetProtocol(protocol Protocol) *Options {
	opts.Protocol = protocol
	return opts
}

func (opts *Options) SetHandler(handler ConnectionHandler) *Options {
	opts.Handler = handler
	return opts
}
