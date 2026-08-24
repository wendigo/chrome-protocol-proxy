package main

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
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

func proxyWS(ctxt context.Context, stream chan *protocolMessage, from, to *websocket.Conn, errc chan error, conn *proxiedConnection, direction string) {
	var mt int
	var buf []byte
	var err error

	for {
		select {
		default:
			mt, buf, err = from.ReadMessage()
			if err != nil {
				errc <- err
				return
			}

			msg, derr := decodeMessage(buf)

			// Responses to commands injected from the UI are consumed here: the
			// proxied client never sent those requests, so they are only shown in
			// the UI and are not forwarded or logged.
			if derr == nil && direction == directionRecv && conn.claimInjectedResponse(msg) {
				hub.publishFrame(conn, direction, buf, true)
				continue
			}

			if derr == nil {
				stream <- msg
			}

			hub.publishFrame(conn, direction, buf, false)

			// Writes towards the browser must go through the connection so they
			// do not interleave with UI command injection.
			if direction == directionSend {
				err = conn.writeToBrowser(mt, buf)
			} else {
				err = to.WriteMessage(mt, buf)
			}

			if err != nil {
				errc <- err
				return
			}

		case <-ctxt.Done():
			return
		}
	}
}
