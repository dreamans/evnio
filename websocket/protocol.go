package websocket

import (
	"bytes"

	"github.com/dreamans/evnio"
)

type Protocol struct{}

func (p *Protocol) UnPacket(c evnio.Connection, buffer *bytes.Buffer) []byte {
	if c.Context().Value(UpgradeContextKey) == nil {
		index := bytes.Index(buffer.Bytes(), []byte("\r\n\r\n"))
		if index == -1 {
			return nil
		}
		buf := buffer.Next(index + 4)
		return buf
	}

	buf := buffer.Bytes()
	buffer.Reset()
	return buf
}

func (p *Protocol) Packet(c evnio.Connection, data []byte) []byte {
	return data
}
