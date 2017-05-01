package main

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
)

const (
	incomingBufferSize = 10 * 1024 * 1024
	outgoingBufferSize = 25 * 1024 * 1024
)

var wsUpgrader = &websocket.Upgrader{
	ReadBufferSize:  incomingBufferSize,
	WriteBufferSize: outgoingBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wsDialer = &websocket.Dialer{
	ReadBufferSize:  outgoingBufferSize,
	WriteBufferSize: incomingBufferSize,
}

func proxyWS(ctxt context.Context, stream chan *protocolMessage, in, out *websocket.Conn, errc chan error) {
	var mt int
	var buf []byte
	var err error

	for {
		select {
		default:
			mt, buf, err = in.ReadMessage()
			if err != nil {
				errc <- err
				return
			}

			if msg, derr := decodeMessage(buf); derr == nil {
				stream <- msg
			}

			err = out.WriteMessage(mt, buf)

			if err != nil {
				errc <- err
				return
			}

		case <-ctxt.Done():
			return
		}
	}
}
