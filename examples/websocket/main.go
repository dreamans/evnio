package main

import (
	"log"

	"github.com/dreamans/evnio"
	"github.com/dreamans/evnio/evlog"
	"github.com/dreamans/evnio/websocket"
)

type Websocket struct{}

func (ws *Websocket) OnOpen(c *websocket.Conn) {
}

func (ws *Websocket) OnMessage(c *websocket.Conn, opCode websocket.OpCode, data []byte) {
}

func (ws *Websocket) OnClose(c *websocket.Conn, closeCode int, closeText string) {
}

func (ws *Websocket) OnPing(c *websocket.Conn, b []byte) {
}

func (ws *Websocket) OnPong(c *websocket.Conn, b []byte) {
}

func (ws *Websocket) OnError(c *websocket.Conn, err error) {
}

func main() {
	evlog.SetLogger(evlog.NewDebugLogger())

	evSrv := evnio.NewServer(&evnio.Options{
		Addr:     ":5100",
		NumLoops: 2,
		Handler: &websocket.Websocket{
			Handler: &Websocket{},
		},
		Protocol: &websocket.Protocol{},
	})

	defer evSrv.Shutdown()

	err := evSrv.Start()
	if err != nil {
		log.Fatal(err)
	}
}
