package websocket

import (
	"context"
	"encoding/binary"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dreamans/evnio"
)

const (
	UpgradeContextKey = "upgrade-context"
)

type Handler interface {
	OnOpen(*Conn)
	OnMessage(*Conn, OpCode, []byte)
	OnClose(*Conn, int, string)
	OnError(*Conn, error)
	OnPing(*Conn, []byte)
	OnPong(*Conn, []byte)
}

type Websocket struct {
	HandshakeTimeout    time.Duration
	MaxFramePayloadSize int
	CheckOrigin         func() bool
	Handler             Handler

	connections sync.Map
}

func (ws *Websocket) OnOpen(c evnio.Connection) {
	if ws.Handler == nil {
		return
	}
	conn := NewConn(c, ws.MaxFramePayloadSize)
	ws.connections.Store(c.UniqID(), conn)
}

func (ws *Websocket) OnMessage(c evnio.Connection, data []byte) {
	if ws.Handler == nil {
		return
	}
	if c.Context().Value(UpgradeContextKey) == nil {
		if err := NewUpgrader(c).Upgrade(data); err != nil {
			_ = c.Close()
			return
		}
		ctx := context.WithValue(c.Context(), UpgradeContextKey, true)
		c.SetContext(ctx)

		conn, ok := ws.connections.Load(c.UniqID())
		if !ok {
			return
		}

		ws.Handler.OnOpen(conn.(*Conn))
		return
	}

	conn, ok := ws.connections.Load(c.UniqID())
	if !ok {
		_ = c.Close()
		return
	}
	cc := conn.(*Conn)
	for {
		// 读取websocket数据帧中数据
		// opCode 可能的返回值
		//		OpNone 			- 解帧报错, 关注err错误
		//		OpWait 			- 缓冲数据不够一个完整帧, 不予处理
		//		OpContinuation 	- 继续帧, 不予处理
		//		OpBinary 		- 二进制数据帧
		//		OpText			- 文本数据帧
		//		OpClose			- 关闭控制帧
		//		OpPing			- Ping帧
		//		OpPong			- Pong帧
		opCode, b, restLen, err := cc.readFrame(data)
		if err != nil {
			ws.Handler.OnError(cc, err)
			return
		}

		switch opCode {
		case OpBinary, OpText:
			ws.Handler.OnMessage(cc, opCode, b)
		case OpPing:
			ws.Handler.OnPing(cc, b)
		case OpPong:
			ws.Handler.OnPong(cc, b)
		case OpClose:
			closeCode := CloseNoStatusReceived
			closeText := ""
			if len(b) >= 2 {
				closeCode = int(binary.BigEndian.Uint16(b))
				closeText = string(b[2:])
				if !utf8.ValidString(closeText) {
					ws.Handler.OnError(cc, handleProtocolError("invalid utf8 payload in close frame"))
				}
			}
			ws.Handler.OnClose(cc, closeCode, closeText)
		}
		if restLen == 0 {
			break
		}
		data = data[0:0]
	}
}

func (ws *Websocket) OnClose(c evnio.Connection) {
	ws.connections.Delete(c.UniqID())
}
