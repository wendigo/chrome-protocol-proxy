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

	"errors"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

var (
	flagListen       = flag.String("l", "localhost:9223", "listen address")
	flagRemote       = flag.String("r", "localhost:9222", "remote address")
	flagNoLog        = flag.Bool("n", false, "disable logging to file")
	flagLogMask      = flag.String("log", "logs/cdp-%s.log", "log file mask")
	flagEllipsis     = flag.Bool("s", false, "shorten requests and responses")
	flagOnce         = flag.Bool("once", false, "debug single session")
	flagShowRequests = flag.Bool("i", false, "include request frames as they are sent")
)

var (
	responseColor     = color.New(color.FgHiGreen).SprintfFunc()
	requestColor      = color.New(color.FgHiBlue).SprintFunc()
	requestReplyColor = color.New(color.FgGreen).SprintfFunc()
	eventsColor       = color.New(color.FgHiRed).SprintfFunc()
	protocolColor     = color.New(color.FgYellow).SprintfFunc()
	protocolError     = color.New(color.FgHiYellow, color.BgRed).SprintfFunc()
	targetColor       = color.New(color.FgHiWhite).SprintfFunc()
	methodColor       = color.New(color.FgHiYellow).SprintfFunc()
	errorColor        = color.New(color.BgRed, color.FgWhite).SprintfFunc()
)

const (
	incomingBufferSize = 10 * 1024 * 1024
	outgoingBufferSize = 25 * 1024 * 1024
	ellipsisLength     = 80
	requestReplyFormat = "%s % 48s(%s) = %s"
	requestFormat      = "%s % 48s(%s)"
	eventFormat        = "%s % 48s(%s)"
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

		stream := make(chan *protocolMessage, 1024)

		id := path.Base(req.URL.Path)
		f, logger := createLog(id)
		if f != nil {
			defer f.Close()
		}
		go dumpStream(logger, stream)

		endpoint := "ws://" + *flagRemote + "/devtools/page/" + id

		logger.Print(protocolColor("---------- connection from %s ----------", req.RemoteAddr))
		logger.Print(protocolColor("checking protocol versions on: %s", endpoint))

		ver, err := checkVersion()
		if err != nil {
			logger.Println(protocolError("could not check version: %v", err))
			http.Error(res, "could not check version", 500)
			return
		}

		logger.Print(protocolColor("protocol version: %s", ver["Protocol-Version"]))
		logger.Print(protocolColor("versions: Chrome(%s), V8(%s), Webkit(%s)", ver["Browser"], ver["V8-Version"], ver["WebKit-Version"]))
		logger.Print(protocolColor("browser user agent: %s", ver["User-Agent"]))

		// connect outgoing websocket
		logger.Print(protocolColor("connecting to %s... ", endpoint))
		out, pres, err := wsDialer.Dial(endpoint, nil)
		if err != nil {
			msg := fmt.Sprintf("could not connect to %s: %v", endpoint, err)
			logger.Println(protocolError(msg))
			http.Error(res, msg, 500)
			return
		}
		defer pres.Body.Close()
		defer out.Close()

		// connect incoming websocket
		logger.Print(protocolColor("upgrading connection on %s...", req.RemoteAddr))
		in, err := wsUpgrader.Upgrade(res, req, nil)
		if err != nil {
			logger.Println(protocolError("could not upgrade websocket from %s: %v", req.RemoteAddr, err))
			http.Error(res, "could not upgrade websocket connection", 500)
			return
		}
		defer in.Close()

		ctxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		errc := make(chan error, 1)
		go proxyWS(ctxt, stream, in, out, errc)
		go proxyWS(ctxt, stream, out, in, errc)
		<-errc
		logger.Printf(protocolColor("---------- closing %s ----------", req.RemoteAddr))

		if *flagOnce {
			os.Exit(0)
		}
	})

	log.Fatal(http.ListenAndServe(*flagListen, mux))
}

func dumpStream(logger *log.Logger, stream chan *protocolMessage) {
	logger.Printf("Legend: %s, %s, %s, %s, %s",
		protocolColor("protocol informations"),
		eventsColor("received events"),
		requestColor("sent request frames"),
		requestReplyColor("requests params"),
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

						if *flagShowRequests {
							logger.Printf(requestFormat, targetColor("%s", msg.Params["targetId"]), methodColor(protocolMessage.Method), requestColor("%s", serialize(protocolMessage)))
						}

					} else {
						logger.Printf(protocolColor("Could not deserialize message: %+v", err))
					}
				}

				if msg.IsEvent() {
					if protocolMessage, err := decodeMessage([]byte(asString(msg.Params["message"]))); err == nil {
						if protocolMessage.IsEvent() {
							logger.Printf(eventFormat, targetColor("%s", msg.Params["targetId"]), methodColor(protocolMessage.Method), eventsColor(serialize(protocolMessage.Params)))
						}

						if protocolMessage.IsResponse() {
							if request, ok := targetRequests[protocolMessage.Id]; ok {
								delete(targetRequests, protocolMessage.Id)

								if protocolMessage.IsError() {
									logger.Printf(requestReplyFormat, targetColor("%s", msg.Params["targetId"]), methodColor(request.Method), requestReplyColor(serialize(request.Params)), errorColor(serialize(protocolMessage.Error)))
								} else {
									logger.Printf(requestReplyFormat, targetColor("%s", msg.Params["targetId"]), methodColor(request.Method), requestReplyColor(serialize(request.Params)), responseColor(serialize(protocolMessage.Result)))
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

					if *flagShowRequests {
						logger.Printf(requestFormat, targetColor(protocolTargetId), methodColor(msg.Method), requestColor(serialize(msg.Params)))
					}
				}

				if msg.IsResponse() {
					if request, ok := requests[msg.Id]; ok {
						delete(requests, msg.Id)

						if request != nil {
							if msg.IsError() {
								logger.Printf(requestReplyFormat, targetColor(protocolTargetId), methodColor(request.Method), requestReplyColor(serialize(request.Params)), errorColor(serialize(msg.Error)))
							} else {
								logger.Printf(requestReplyFormat, targetColor(protocolTargetId), methodColor(request.Method), requestReplyColor(serialize(request.Params)), responseColor(serialize(msg.Result)))
							}
						}
					} else {
						logger.Printf(protocolColor("Could not find request with id: %d", msg.Id))
					}
				}

				if msg.IsEvent() {
					logger.Printf(eventFormat, targetColor(protocolTargetId), methodColor(msg.Method), eventsColor(serialize(msg.Params)))
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
	req, err := http.NewRequest("GET", "http://"+*flagRemote+"/json/version", nil)
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
		return nil, errors.New("expected json result")
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
