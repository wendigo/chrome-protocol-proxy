package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakeBrowser mimics a browser exposing the DevTools protocol: it answers
// /json/version and echoes a result for every command received on the websocket.
func fakeBrowser(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/json/version", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]string{
			"Protocol-Version": "1.3",
			"Browser":          "FakeChrome/1.0",
		})
	})

	mux.HandleFunc("/devtools/page/", func(res http.ResponseWriter, req *http.Request) {
		c, err := wsUpgrader.Upgrade(res, req, nil)
		if err != nil {
			t.Errorf("fake browser could not upgrade: %v", err)
			return
		}
		defer c.Close()

		for {
			var msg protocolMessage
			if err := c.ReadJSON(&msg); err != nil {
				return
			}

			reply := map[string]interface{}{
				"id":     msg.ID,
				"result": map[string]interface{}{"method": msg.Method},
			}

			if err := c.WriteJSON(reply); err != nil {
				return
			}
		}
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

func wsURL(server *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + path
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	c, res, err := wsDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("could not dial %s: %v", url, err)
	}
	if res != nil {
		defer res.Body.Close()
	}
	t.Cleanup(func() { c.Close() })

	return c
}

// readUIMessage reads messages from the UI websocket until predicate matches.
func readUIMessage(t *testing.T, c *websocket.Conn, what string, predicate func(map[string]interface{}) bool) map[string]interface{} {
	t.Helper()

	c.SetReadDeadline(time.Now().Add(5 * time.Second))

	for {
		var msg map[string]interface{}
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatalf("did not receive %s: %v", what, err)
		}

		if predicate(msg) {
			return msg
		}
	}
}

func TestInteractiveUI(t *testing.T) {
	browser := fakeBrowser(t)

	*flagRemote = strings.TrimPrefix(browser.URL, "http://")
	*flagQuiet = true
	*flagDirLogs = t.TempDir()
	*flagUI = true

	proxy := httptest.NewServer(createMux())
	t.Cleanup(proxy.Close)

	// serves the embedded UI page
	res, err := http.Get(proxy.URL + "/ui")
	if err != nil {
		t.Fatalf("could not fetch /ui: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /ui status: %d", res.StatusCode)
	}

	ui := dialWS(t, wsURL(proxy, "/ui/ws"))
	client := dialWS(t, wsURL(proxy, "/devtools/page/test-page"))

	// UI is told about the proxied connection
	var connID string
	readUIMessage(t, ui, "connections message", func(msg map[string]interface{}) bool {
		if msg["type"] != "connections" {
			return false
		}
		conns, _ := msg["connections"].([]interface{})
		if len(conns) == 0 {
			return false
		}
		connID = conns[0].(map[string]interface{})["id"].(string)
		return true
	})

	// a frame sent by the client is streamed to the UI and forwarded to the browser
	if err := client.WriteJSON(map[string]interface{}{"id": 1, "method": "Page.enable"}); err != nil {
		t.Fatalf("client could not send: %v", err)
	}

	var requestBoot string
	var requestSeq float64
	requestFrame := readUIMessage(t, ui, "client request frame", func(msg map[string]interface{}) bool {
		if msg["type"] != "frame" || msg["direction"] != directionSend {
			return false
		}
		message := msg["message"].(map[string]interface{})
		return message["method"] == "Page.enable"
	})

	requestBoot, _ = requestFrame["boot"].(string)
	requestSeq, _ = requestFrame["seq"].(float64)

	if requestBoot == "" || requestSeq == 0 {
		t.Fatalf("frame is missing boot/seq stamps: %+v", requestFrame)
	}

	// the browser response reaches both the client and the UI
	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	var clientReply protocolMessage
	if err := client.ReadJSON(&clientReply); err != nil {
		t.Fatalf("client did not receive response: %v", err)
	}
	if clientReply.ID != 1 {
		t.Fatalf("client received unexpected response: %+v", clientReply)
	}

	responseFrame := readUIMessage(t, ui, "browser response frame", func(msg map[string]interface{}) bool {
		return msg["type"] == "frame" && msg["direction"] == directionRecv
	})

	if seq, _ := responseFrame["seq"].(float64); seq <= requestSeq {
		t.Fatalf("frame seq is not monotonic: request=%v response=%v", requestSeq, seq)
	}

	// a command injected from the UI reaches the browser, its response comes back
	// to the UI marked as injected
	command := map[string]interface{}{
		"type":   "send",
		"connId": connID,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{"expression": "1+1"},
	}
	if err := ui.WriteJSON(command); err != nil {
		t.Fatalf("ui could not send command: %v", err)
	}

	var injectedID float64
	readUIMessage(t, ui, "injected request frame", func(msg map[string]interface{}) bool {
		if msg["type"] != "frame" || msg["injected"] != true || msg["direction"] != directionSend {
			return false
		}
		message := msg["message"].(map[string]interface{})
		injectedID = message["id"].(float64)
		return message["method"] == "Runtime.evaluate"
	})

	if uint64(injectedID) < injectedIDBase {
		t.Fatalf("injected command id %d is below the reserved range", uint64(injectedID))
	}

	readUIMessage(t, ui, "injected response frame", func(msg map[string]interface{}) bool {
		if msg["type"] != "frame" || msg["injected"] != true || msg["direction"] != directionRecv {
			return false
		}
		message := msg["message"].(map[string]interface{})
		return message["id"] == injectedID
	})

	// the proxied client must never see the injected response
	client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var unexpected protocolMessage
	if err := client.ReadJSON(&unexpected); err == nil {
		t.Fatalf("client received a frame it should not have: %+v", unexpected)
	}

	// errors are reported back to the UI
	if err := ui.WriteJSON(map[string]interface{}{"type": "send", "connId": "nope", "method": "Page.enable"}); err != nil {
		t.Fatalf("ui could not send command: %v", err)
	}

	readUIMessage(t, ui, "error message", func(msg map[string]interface{}) bool {
		return msg["type"] == "error" && strings.Contains(fmt.Sprint(msg["message"]), "nope")
	})

	// a reconnecting UI client receives the recent buffer with the same boot and
	// seq stamps, so it can skip frames it already has
	reconnected := dialWS(t, wsURL(proxy, "/ui/ws"))

	replayed := readUIMessage(t, reconnected, "replayed request frame", func(msg map[string]interface{}) bool {
		if msg["type"] != "frame" {
			return false
		}
		message := msg["message"].(map[string]interface{})
		return message["method"] == "Page.enable" && msg["direction"] == directionSend
	})

	if boot, _ := replayed["boot"].(string); boot != requestBoot {
		t.Fatalf("replayed frame boot mismatch: %q != %q", boot, requestBoot)
	}

	if seq, _ := replayed["seq"].(float64); seq != requestSeq {
		t.Fatalf("replayed frame seq mismatch: %v != %v", seq, requestSeq)
	}
}
