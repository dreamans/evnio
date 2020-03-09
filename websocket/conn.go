package websocket

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/dreamans/evnio"
)

type OpCode byte

const (
	OpContinuation OpCode = 0x0
	OpText         OpCode = 0x1
	OpBinary       OpCode = 0x2
	OpClose        OpCode = 0x8
	OpPing         OpCode = 0x9
	OpPong         OpCode = 0xA
	OpWait         OpCode = 0xFE
	OpNone         OpCode = 0xFF
)

const (
	CloseNormalClosure           = 1000
	CloseGoingAway               = 1001
	CloseProtocolError           = 1002
	CloseUnsupportedData         = 1003
	CloseNoStatusReceived        = 1005
	CloseAbnormalClosure         = 1006
	CloseInvalidFramePayloadData = 1007
	ClosePolicyViolation         = 1008
	CloseMessageTooBig           = 1009
	CloseMandatoryExtension      = 1010
	CloseInternalServerErr       = 1011
	CloseServiceRestart          = 1012
	CloseTryAgainLater           = 1013
)

func (op OpCode) isControl() bool {
	return op == OpClose || op == OpPing || op == OpPong
}

func (op OpCode) isData() bool {
	return op == OpText || op == OpBinary
}

const (
	finalBit = 1 << 7
	rsv1Bit  = 1 << 6
	rsv2Bit  = 1 << 5
	rsv3Bit  = 1 << 4
	maskBit  = 1 << 7
)

const (
	defaultMaxPayloadSize      = 4096
	maxControlFramePayloadSize = 125
)

type Conn struct {
	conn             evnio.Connection
	maxPayloadSize   int
	segmentBuf       []byte
	readBuf          []byte
	readRemaining    int64
	readFinal        bool
	readLastFinal    bool
	readHeaderSize   int64
	opCode           OpCode
	multiFrameOpCode OpCode
	maskingKey       [4]byte
}

var (
	ErrReadLimit           = errors.New("websocket: read limit exceeded")
	ErrBadWriteOpCode      = errors.New("websocket: bad write message type")
	errInvalidControlFrame = errors.New("websocket: invalid control frame")
)

func NewConn(c evnio.Connection, maxPayloadSize int) *Conn {
	if maxPayloadSize == 0 {
		maxPayloadSize = defaultMaxPayloadSize
	}
	return &Conn{
		maxPayloadSize:   maxPayloadSize,
		conn:             c,
		readLastFinal:    true,
		multiFrameOpCode: OpNone,
	}
}

func (c *Conn) readFrameHeader() error {
	c.readHeaderSize = 0

	if len(c.segmentBuf) < 2 {
		return nil
	}

	c.readFinal = c.segmentBuf[0]&finalBit != 0
	c.opCode = OpCode(c.segmentBuf[0] & 0xF)
	_ = c.setReadRemaining(int64(c.segmentBuf[1] & 0x7F))

	switch c.opCode {
	case OpClose, OpPing, OpPong:
		if c.readRemaining > 125 {
			return handleProtocolError("control frame length > 125")
		}
		if !c.readFinal {
			return handleProtocolError("control frame not final")
		}
	case OpText, OpBinary:
		if !c.readLastFinal {
			return handleProtocolError("message start before final message frame")
		}
		c.readLastFinal = c.readFinal
	case OpContinuation:
		if c.readLastFinal {
			return handleProtocolError("continuation after final message frame")
		}
		c.readLastFinal = c.readFinal
	default:
		return handleProtocolError(fmt.Sprintf("unknown opcode %b", c.opCode))
	}

	if rsv := c.segmentBuf[0] & (rsv1Bit | rsv2Bit | rsv3Bit); rsv != 0 {
		return handleProtocolError("unexpected reserved bits 0x" + strconv.FormatInt(int64(rsv), 16))
	}

	if isMask := c.segmentBuf[1] & maskBit; isMask == 0 {
		return handleProtocolError("incorrect mask flag")
	}

	headerSize := int64(2)
	switch c.readRemaining {
	case 126:
		if len(c.segmentBuf) < 4 {
			return nil
		}
		if err := c.setReadRemaining(int64(binary.BigEndian.Uint16(c.segmentBuf[2:4]))); err != nil {
			return err
		}
		headerSize = 4
	case 127:
		if len(c.segmentBuf) < 10 {
			return nil
		}
		if err := c.setReadRemaining(int64(binary.BigEndian.Uint64(c.segmentBuf[2:10]))); err != nil {
			return err
		}
		headerSize = 10
	}

	//Masking-Key
	if len(c.segmentBuf) < int(headerSize)+4 {
		return nil
	}
	copy(c.maskingKey[:], c.segmentBuf[headerSize:headerSize+4])
	headerSize += 4

	c.readHeaderSize = headerSize

	// 多帧中的第一帧, 提取数据类型
	if (c.opCode == OpText || c.opCode == OpBinary) && !c.readFinal {
		c.multiFrameOpCode = c.opCode
	}

	return nil
}

func (c *Conn) readFramePayload() (OpCode, []byte, int) {
	c.segmentBuf = c.segmentBuf[c.readHeaderSize:]
	if c.readRemaining > int64(len(c.segmentBuf)) {
		return OpWait, nil, 0
	}

	payload := c.segmentBuf[:c.readRemaining]
	maskBytes(c.maskingKey, payload)
	c.segmentBuf = c.segmentBuf[c.readRemaining:]

	if !c.readFinal || c.opCode == OpContinuation {
		c.readBuf = append(c.readBuf, payload...)
	}

	// 多帧中非最后一帧
	if !c.readFinal {
		return OpContinuation, nil, len(c.segmentBuf)
	}

	// 多帧中最后一帧
	if c.readFinal && c.opCode == OpContinuation {
		opCode, b := c.multiFrameOpCode, c.readBuf
		// TODO: check multiFrameOpCode == OpNone error

		c.multiFrameOpCode = OpNone
		c.readBuf = c.readBuf[0:0]

		return opCode, b, len(c.segmentBuf)
	}

	return c.opCode, payload, len(c.segmentBuf)
}

func (c *Conn) readFrame(data []byte) (OpCode, []byte, int, error) {
	c.segmentBuf = append(c.segmentBuf, data...)

	if c.readHeaderSize == 0 {
		if err := c.readFrameHeader(); err != nil {
			return OpNone, nil, 0, err
		}
	}

	if c.readHeaderSize > 0 {
		opCode, b, restLen := c.readFramePayload()
		if opCode != OpWait {
			c.readHeaderSize = 0
		}
		return opCode, b, restLen, nil
	}

	return OpWait, nil, 0, nil
}

func (c *Conn) setReadRemaining(n int64) error {
	if n < 0 {
		return ErrReadLimit
	}

	c.readRemaining = n
	return nil
}

func (c *Conn) writeFrame(final bool, opCode OpCode, data []byte) error {
	var b0, b1 byte

	if final {
		b0 |= finalBit
	}
	b0 |= byte(opCode)

	headerBuf, headerPos := make([]byte, 10), 0
	headerBuf[0] = b0
	headerPos++

	length := len(data)
	switch {
	case length > 65535:
		headerPos += 9
		headerBuf[1] = b1 | 127
		binary.BigEndian.PutUint64(headerBuf[2:], uint64(length))
	case length > 125:
		headerPos += 3
		headerBuf[1] = b1 | 126
		binary.BigEndian.PutUint16(headerBuf[2:], uint16(length))
	default:
		headerPos += 1
		headerBuf[1] = b1 | byte(length)
	}

	frame := make([]byte, len(data)+headerPos)
	copy(frame[:headerPos], headerBuf[:headerPos])
	copy(frame[headerPos:], data)
	return c.conn.Send(frame, evnio.ActionNone)
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) WriteMessage(opCode OpCode, data []byte) error {
	if len(data) <= c.maxPayloadSize {
		return c.writeFrame(true, opCode, data)
	}

	var posBegin, posFinish int
	final, length, originOpCode := false, len(data), opCode
	for {
		if length-posBegin <= c.maxPayloadSize {
			posFinish, final = length, true
		} else {
			posFinish = posBegin + c.maxPayloadSize
		}
		if err := c.writeFrame(final, originOpCode, data[posBegin:posFinish]); err != nil {
			return err
		}
		if posFinish == length {
			break
		}
		originOpCode = OpContinuation
		posBegin += c.maxPayloadSize
	}
	return nil
}

func (c *Conn) writeControl(opCode OpCode, b []byte) error {
	if len(b) > maxControlFramePayloadSize {
		return errInvalidControlFrame
	}
	return c.writeFrame(true, OpClose, b)
}

func (c *Conn) WriteCloseMessage(closeCode int, text string) error {
	if closeCode == CloseNoStatusReceived {
		return nil
	}
	buf := make([]byte, 2+len(text))
	binary.BigEndian.PutUint16(buf, uint16(closeCode))
	copy(buf[2:], text)
	return c.writeControl(OpClose, buf)
}

func (c *Conn) WritePingMessage(b []byte) error {
	return c.writeControl(OpPing, b)
}

func (c *Conn) WritePongMessage(b []byte) error {
	return c.writeControl(OpPong, b)
}
