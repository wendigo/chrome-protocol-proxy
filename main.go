// This software is direct fork of https://github.com/knq/chromedp/tree/master/cmd/chromedp-proxy
// with couple of features added
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"

	"github.com/gorilla/websocket"
	"github.com/fatih/color"
)

var (
	flagListen   = flag.String("l", "localhost:9223", "listen address")
	flagRemote   = flag.String("r", "localhost:9222", "remote address")
	flagNoLog    = flag.Bool("n", false, "disable logging to file")
	flagLogMask  = flag.String("log", "logs/cdp-%s.log", "log file mask")
	flagEllipsis = flag.Bool("s", false, "shorten requests and responses")
)


var (
	responseColor = color.New(color.FgHiGreen).SprintfFunc()
	requestColor = color.New(color.FgGreen).SprintfFunc()
	eventsColor = color.New(color.FgHiRed).SprintfFunc()
	protocolColor = color.New(color.FgYellow).SprintfFunc()
	protocolError = color.New(color.FgHiYellow, color.BgRed).SprintfFunc()
	targetColor = color.New(color.FgHiWhite).SprintfFunc()
	methodColor = color.New(color.FgHiYellow).SprintfFunc()
	errorColor = color.New(color.BgRed, color.FgWhite).SprintfFunc()
)

const (
	incomingBufferSize = 10 * 1024 * 1024
	outgoingBufferSize = 25 * 1024 * 1024
	ellipsisLength     = 80
)

var protocolTargetId = center("protocol message", 36)

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


func main() {
	flag.Parse()

	mux := http.NewServeMux()

	simplep := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: *flagRemote})

	mux.Handle("/json", simplep)
	mux.Handle("/", simplep)

	mux.HandleFunc("/devtools/page/", func(res http.ResponseWriter, req *http.Request) {

		id := path.Base(req.URL.Path)
		f, logger := createLog(id)
		if f != nil {
			defer f.Close()
		}

		stream := make(chan *protocolMessage, 1024)
		go dumpStream(logger, stream)

		logger.Printf(protocolColor("---------- connection from %s ----------", req.RemoteAddr))

		ver, err := checkVersion()
		if err != nil {
			msg := fmt.Sprintf("version error, got: %v", err)
			logger.Println(protocolError(msg))
			http.Error(res, msg, 500)
			return
		}
		logger.Printf(protocolColor("endpoint %s reported: %s", *flagRemote, serialize(ver)))

		endpoint := "ws://" + *flagRemote + "/devtools/page/" + id

		// connect outgoing websocket
		logger.Printf(protocolColor("connecting to %s", endpoint))
		out, pres, err := wsDialer.Dial(endpoint, nil)
		if err != nil {
			msg := fmt.Sprintf("could not connect to %s, got: %v", endpoint, err)
			logger.Println(protocolError(msg))
			http.Error(res, msg, 500)
			return
		}
		defer pres.Body.Close()
		defer out.Close()

		logger.Printf(protocolColor("connected to %s", endpoint))

		// connect incoming websocket
		logger.Printf(protocolColor("upgrading connection on %s", req.RemoteAddr))
		in, err := wsUpgrader.Upgrade(res, req, nil)
		if err != nil {
			msg := fmt.Sprintf("could not upgrade websocket from %s, got: %v", req.RemoteAddr, err)
			logger.Println(protocolError(msg))
			http.Error(res, msg, 500)
			return
		}
		defer in.Close()
		logger.Printf(protocolColor("upgraded connection on %s", req.RemoteAddr))

		ctxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		errc := make(chan error, 1)
		go proxyWS(ctxt, stream, in, out, errc)
		go proxyWS(ctxt, stream, out, in, errc)
		<-errc
		logger.Printf(protocolColor("---------- closing %s ----------", req.RemoteAddr))
	})


	log.Fatal(http.ListenAndServe(*flagListen, mux))
}

func dumpStream(logger *log.Logger, stream chan *protocolMessage) {
	logger.Printf("Legend: %s, %s, %s, %s",
		protocolColor("protocol informations"),
		eventsColor("received events"),
		requestColor("sent requests"),
		responseColor("received responses."),
	)

	requests := make(map[uint64]*protocolMessage)
	targetRequests := make(map[uint64]*protocolMessage)

	for {
		select {
		case msg := <-stream:
			if msg.InTarget() {
				if msg.IsRequest() {
					requests[msg.Id] = nil

					if protocolMessage, err := decodeMessage([]byte(asString(msg.Params["message"]))); err == nil {
						targetRequests[protocolMessage.Id] = protocolMessage
					} else {
						logger.Printf(protocolColor("Could not deserialize message: %+v", err))
					}
				}

				if msg.IsEvent() {
					if protocolMessage, err := decodeMessage([]byte(asString(msg.Params["message"]))); err == nil {
						if protocolMessage.IsEvent() {
							logger.Printf("%s %36s <- %s", targetColor("%s", msg.Params["targetId"]), methodColor(protocolMessage.Method), eventsColor(serialize(protocolMessage.Params)))
						}

						if protocolMessage.IsResponse() {
							if request, ok := targetRequests[protocolMessage.Id]; ok {
								delete(targetRequests, protocolMessage.Id)

								if protocolMessage.IsError() {
									logger.Printf("%s %36s(%s) = %s", targetColor("%s", msg.Params["targetId"]), methodColor(request.Method), requestColor(serialize(request.Params)), errorColor(serialize(protocolMessage.Error)))
								} else {
									logger.Printf("%s %36s(%s) = %s", targetColor("%s", msg.Params["targetId"]), methodColor(request.Method), requestColor(serialize(request.Params)), responseColor(serialize(protocolMessage.Result)))
								}
							} else {
								logger.Printf(protocolColor("Could not find target request with id: %d", protocolMessage.Id))
							}
						}
					} else {
						logger.Printf(protocolColor("Could not deserialize message: %+v", err))
					}
				}

			} else {
				if msg.IsRequest() {
					requests[msg.Id] = msg
				}

				if msg.IsResponse() {
					if request, ok := requests[msg.Id]; ok {
						delete(requests, msg.Id)
						
						if request != nil {
							if msg.IsError() {
								logger.Printf("%s %36s(%s) = %s", targetColor(protocolTargetId), methodColor(request.Method), requestColor(serialize(request.Params)), errorColor(serialize(msg.Error)))
							} else {
								logger.Printf("%s %36s(%s) = %s", targetColor(protocolTargetId), methodColor(request.Method), requestColor(serialize(request.Params)), responseColor(serialize(msg.Result)))
							}
						}
					} else {
					logger.Printf(protocolColor("Could not find request with id: %d", msg.Id))
					}
				}

				if msg.IsEvent() {
					logger.Printf("%s %36s <- %s", targetColor(protocolTargetId), methodColor(msg.Method), eventsColor(serialize(msg.Params)))
				}
			}
		}
	}
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

			if msg, err := decodeMessage(buf); err == nil {
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

func checkVersion() (map[string]string, error) {
	cl := &http.Client{}
	req, err := http.NewRequest("GET", "http://" + *flagRemote + "/json/version", nil)
	if err != nil {
		return nil, err
	}

	res, err := cl.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var v map[string]string
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("expected json result")
	}

	return v, nil
}

func createLog(id string) (io.Closer, *log.Logger) {
	var f io.Closer
	var w io.Writer = os.Stdout
	if !*flagNoLog && *flagLogMask != "" {
		l, err := os.OpenFile(fmt.Sprintf(*flagLogMask, id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}

		f = l
		w = io.MultiWriter(os.Stdout, l)
	}
	return f, log.New(w, "", log.Lmicroseconds)
}

