package evnio

import (
	"bytes"
)

type Protocol interface {
	UnPacket(Connection, *bytes.Buffer) []byte
	Packet(Connection, []byte) []byte
}

type defaultProtocol struct{}

func (d *defaultProtocol) UnPacket(c Connection, buffer *bytes.Buffer) []byte {
	buf := buffer.Bytes()
	buffer.Reset()
	return buf
}

func (d *defaultProtocol) Packet(c Connection, data []byte) []byte {
	return data
}
