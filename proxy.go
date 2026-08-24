package main

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// wsBufferSize is only the I/O chunk size: messages larger than the buffer are
// read and written in multiple chunks by gorilla/websocket.
const wsBufferSize = 64 * 1024

var wsUpgrader = &websocket.Upgrader{
	ReadBufferSize:  wsBufferSize,
	WriteBufferSize: wsBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wsDialer = &websocket.Dialer{
	ReadBufferSize:  wsBufferSize,
	WriteBufferSize: wsBufferSize,
}

func proxyWS(stream chan *protocolMessage, from, to *websocket.Conn, errc chan error, conn *proxiedConnection, direction string) {
	for {
		mt, buf, err := from.ReadMessage()
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
	}
}
