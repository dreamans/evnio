package websocket

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dreamans/evnio"
)

type Upgrader struct {
	c evnio.Connection
}

func NewUpgrader(c evnio.Connection) *Upgrader {
	return &Upgrader{c}
}

func (u *Upgrader) Upgrade(data []byte) error {
	return u.upgrade(data)
}

func (u *Upgrader) upgrade(data []byte) error {
	lines := bytes.Split(data, []byte("\r\n"))
	if len(lines) == 0 {
		return u.returnHandshakeError(http.StatusBadRequest, "len(lines) = 0")
	}
	indexMethod := bytes.IndexByte(lines[0], ' ')
	if indexMethod == -1 || string(lines[0][:indexMethod]) != "GET" {
		return u.returnHandshakeError(http.StatusMethodNotAllowed, "request method is not GET")
	}

	indexUri := bytes.IndexByte(lines[0][indexMethod+1:], ' ')
	if indexUri == -1 {
		return u.returnHandshakeError(http.StatusBadRequest, "request uri invalid")
	}
	uri := lines[0][indexMethod+1:][:indexUri]

	_, _ = url.Parse(string(uri))

	headers := map[string]string{}
	for i := 1; i < len(lines); i++ {
		index := bytes.IndexByte(lines[i], ':')
		if index == -1 {
			continue
		}
		k := strings.ToLower(string(bytes.TrimSpace(lines[i][:index])))
		headers[k] = string(bytes.TrimSpace(lines[i][index+1:]))
	}

	if !u.headerCheckExists(headers, "Connection", "upgrade") {
		return u.returnHandshakeError(http.StatusBadRequest, "'upgrade' token not found in 'Connection' header")
	}

	if !u.headerCheckExists(headers, "Upgrade", "websocket") {
		return u.returnHandshakeError(http.StatusBadRequest, "'websocket' token not found in 'Upgrade' header")
	}

	if !u.headerCheckExists(headers, "Sec-Websocket-Version", "13") {
		return u.returnHandshakeError(http.StatusBadRequest, "websocket: unsupported version: 13 not found in 'Sec-Websocket-Version' header")
	}

	challengeKey := headers["sec-websocket-key"]
	if challengeKey == "" {
		return u.returnHandshakeError(http.StatusBadRequest, "websocket: not a websocket handshake: 'Sec-WebSocket-Key' header is missing or blank")
	}

	headers = map[string]string{
		"Connection":           "upgrade",
		"Upgrade":              "websocket",
		"Sec-WebSocket-Accept": u.computeAcceptKey(challengeKey),
	}
	return u.httpRender(http.StatusSwitchingProtocols, headers, nil, false)
}

func (u *Upgrader) returnHandshakeError(status int, message string) error {
	headers := map[string]string{}
	headers["Content-Type"] = "text/plain; charset=UTF-8"
	headers["X-Content-Type-Options"] = "nosniff"
	headers["Sec-Websocket-Version"] = "13"

	_ = u.httpRender(status, headers, []byte(message), true)

	return errors.New(message)
}

var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func (u *Upgrader) computeAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (u *Upgrader) headerCheckExists(headers map[string]string, key, value string) bool {
	key = strings.ToLower(key)
	header, ok := headers[key]
	if !ok {
		return false
	}
	if strings.ToLower(header) != value {
		return false
	}
	return true
}

func (u *Upgrader) httpRender(status int, headers map[string]string, body []byte, close bool) error {
	headers["Content-Length"] = strconv.Itoa(len(body))
	headers["Date"] = time.Now().UTC().Format(http.TimeFormat)

	var b []byte
	b = append(b, "HTTP/1.1 "...)
	b = append(b, strconv.Itoa(status)...)
	b = append(b, ' ')
	b = append(b, http.StatusText(status)...)
	b = append(b, "\r\n"...)

	for k, v := range headers {
		b = append(b, k...)
		b = append(b, ": "...)
		b = append(b, v...)
		b = append(b, "\r\n"...)
	}

	b = append(b, "\r\n"...)
	b = append(b, body...)

	if close {
		return u.c.Send(b, evnio.ActionClose)
	} else {
		return u.c.Send(b, evnio.ActionNone)
	}
}
