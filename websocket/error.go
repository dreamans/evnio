package websocket

import "errors"

func handleProtocolError(message string) error {
	return errors.New("websocket: " + message)
}
