// This software is direct fork of https://github.com/knq/chromedp/tree/master/cmd/chromedp-proxy
// with couple of features added
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	version string = "unknown"
	commit  string = "unknown"
	date    string = "unknown"
	builtBy string = "unknown"
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Printf("%s version %s built on %s by %s\n\nConfiguration:\n", os.Args[0], version, date, builtBy)
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Proxy is listening for DevTools connections on: %s", *flagListen)

	if *flagUI {
		log.Printf("Interactive UI is available on: http://%s/ui", *flagListen)
	}

	log.Fatal(http.ListenAndServe(*flagListen, createMux()))
}

func createMux() *http.ServeMux {
	mux := http.NewServeMux()

	simpleReverseProxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: *flagRemote})

	mux.Handle("/json", simpleReverseProxy)
	mux.Handle("/", simpleReverseProxy)

	rootLogger, err := createLogger("connection")
	if err != nil {
		panic(fmt.Sprintf("could not create logger: %s", err))
	}

	logger := rootLogger.WithFields(logrus.Fields{
		fieldLevel: levelConnection,
	})

	handlerFunc := func(basePath string) func(http.ResponseWriter, *http.Request) {
		return func(res http.ResponseWriter, req *http.Request) {

			stream := make(chan *protocolMessage, 1024)
			id := strings.ReplaceAll(strings.TrimPrefix(req.URL.Path, "/devtools/"), "/", "-")

			var protocolLogger *logrus.Entry

			if *flagDistributeLogs {
				logger, err := createLogger(id)
				if err != nil {
					panic(fmt.Sprintf("could not create logger: %s", err))
				}

				protocolLogger = logger.WithFields(logrus.Fields{
					fieldLevel:       levelConnection,
					fieldInspectorID: id,
				})

			} else {
				protocolLogger = logger.WithFields(logrus.Fields{
					fieldInspectorID: id,
				})
			}

			go dumpStream(protocolLogger, stream)

			endpoint := "ws://" + *flagRemote + "/devtools/" + basePath + "/" + path.Base(req.URL.Path)

			logger.Infof("---------- connection from %s to %s ----------", req.RemoteAddr, req.RequestURI)
			logger.Infof("checking protocol versions on: %s", endpoint)

			ver, err := checkVersion()
			if err != nil {
				protocolLogger.Errorf("could not check version: %v", err)
				http.Error(res, "could not check version", 500)
				return
			}

			logger.Infof("protocol version: %s", ver["Protocol-Version"])
			logger.Infof("versions: Chrome(%s), V8(%s), Webkit(%s)", ver["Browser"], ver["V8-Version"], ver["WebKit-Version"])
			logger.Infof("browser user agent: %s", ver["User-Agent"])
			logger.Infof("connecting to %s... ", endpoint)

			// connecting to ws
			out, pres, err := wsDialer.Dial(endpoint, nil)
			if err != nil {
				msg := fmt.Sprintf("could not connect to %s: %v", endpoint, err)
				logger.Error(protocolError(msg))
				http.Error(res, msg, 500)
				return
			}
			defer pres.Body.Close()
			defer out.Close()

			conn := hub.register(id, endpoint, out)
			defer hub.unregister(conn)

			// connect incoming websocket
			logger.Infof("upgrading connection on %s...", req.RemoteAddr)
			in, err := wsUpgrader.Upgrade(res, req, nil)
			if err != nil {
				logger.Errorf("could not upgrade websocket from %s: %v", req.RemoteAddr, err)
				http.Error(res, "could not upgrade websocket connection", 500)
				return
			}
			defer in.Close()

			errc := make(chan error, 1)
			go proxyWS(stream, in, out, errc, conn, directionSend)
			go proxyWS(stream, out, in, errc, conn, directionRecv)

			<-errc
			close(stream)

			logger.Infof("---------- closing connection from %s to %s ----------", req.RemoteAddr, req.RequestURI)

			if *flagDistributeLogs {
				destroyLogger(id)
			}

			if *flagOnce {
				os.Exit(0)
			}
		}
	}

	mux.HandleFunc("/devtools/page/", handlerFunc("page"))
	mux.HandleFunc("/devtools/browser/", handlerFunc("browser"))

	if *flagUI {
		mux.HandleFunc("/ui", serveUI)
		mux.HandleFunc("/ui/ws", uiWSHandler)
	}

	return mux
}

// logRequest logs a request frame when -i is enabled.
func logRequest(logger *logrus.Entry, msg *protocolMessage) {
	if !*flagShowRequests {
		return
	}

	logger.WithFields(logrus.Fields{
		fieldType:   typeRequest,
		fieldMethod: msg.Method + "-(" + strconv.FormatUint(msg.ID, 10) + ")",
	}).Info(serialize(msg.Params))
}

// logResponse logs a response frame coalesced with its originating request,
// which may be nil when it was never seen.
func logResponse(logger *logrus.Entry, request *protocolMessage, msg *protocolMessage) {
	logMessage := serialize(msg.Result)
	logType := typeRequestResponse

	if msg.IsError() {
		logMessage = serialize(msg.Error)
		logType = typeRequestResponseError
	}

	var requestText, method string

	if request != nil {
		requestText = serialize(request.Params)
		method = request.Method
	} else {
		requestText = errorColor("could not find request with id: %d", msg.ID)
	}

	if *flagShowRequests {
		method += "*(" + strconv.FormatUint(msg.ID, 10) + ")"
	} else {
		method += "*"
	}

	logger.WithFields(logrus.Fields{
		fieldType:    logType,
		fieldMethod:  method,
		fieldRequest: requestText,
	}).Info(logMessage)
}

func dumpStream(logger *logrus.Entry, stream chan *protocolMessage) {
	logger.Printf("Legend: %s, %s, %s, %s, %s, %s", protocolColor("protocol informations"),
		eventsColor("received events"),
		requestColor("sent request frames"),
		requestReplyColor("requests params"),
		responseColor("received responses"),
		errorColor("error response."),
	)

	requests := make(map[uint64]*protocolMessage)
	sessions := make(map[string]map[uint64]*protocolMessage)

	deserializationError := func(err error) {
		logger.WithFields(logrus.Fields{
			fieldLevel: levelConnection,
		}).Errorf("Could not deserialize message: %+v", err)
	}

	for msg := range stream {
		if msg.HasSessionId() {
			targetID := msg.TargetID()

			targetRequests, exists := sessions[targetID]
			if !exists {
				targetRequests = make(map[uint64]*protocolMessage)
				sessions[targetID] = targetRequests
			}

			var fieldLogger logrus.FieldLogger = logger

			if *flagDistributeLogs {
				sessionLogger, err := createLogger("session-" + targetID)
				if err != nil {
					panic(fmt.Sprintf("could not create logger: %v", err))
				}

				fieldLogger = sessionLogger
			}

			targetLogger := fieldLogger.WithFields(logrus.Fields{
				fieldLevel:    levelTarget,
				fieldTargetID: targetID,
			})

			switch {
			case msg.IsRequest():
				// The placeholder suppresses logging of the top-level ack that
				// the browser sends for the carrying request.
				requests[msg.ID] = nil

				if protocolMessage, err := decodeProtocolMessage(msg); err == nil {
					targetRequests[protocolMessage.ID] = protocolMessage
					logRequest(targetLogger, protocolMessage)
				} else {
					deserializationError(err)
				}

			case msg.IsEvent():
				if protocolMessage, err := decodeProtocolMessage(msg); err == nil {
					switch {
					case protocolMessage.IsEvent():
						targetLogger.WithFields(logrus.Fields{
							fieldType:   typeEvent,
							fieldMethod: protocolMessage.Method,
						}).Info(serialize(protocolMessage.Params))

					case protocolMessage.IsResponse():
						request := targetRequests[protocolMessage.ID]
						delete(targetRequests, protocolMessage.ID)
						logResponse(targetLogger, request, protocolMessage)

					default:
						targetLogger.WithFields(logrus.Fields{
							fieldType:   typeRequest,
							fieldMethod: msg.Method,
						}).Info("Could not understand session event: " + msg.raw)
					}
				} else {
					deserializationError(err)
				}

			case msg.IsResponse():
				request := targetRequests[msg.ID]
				delete(targetRequests, msg.ID)
				logResponse(targetLogger, request, msg)

			default:
				targetLogger.WithFields(logrus.Fields{
					fieldType:   typeRequest,
					fieldMethod: msg.Method,
				}).Info("Could not understand session message: " + msg.raw)
			}

		} else {
			protocolLogger := logger.WithFields(logrus.Fields{
				fieldLevel:    levelProtocol,
				fieldTargetID: protocolTargetID,
			})

			switch {
			case msg.IsRequest():
				requests[msg.ID] = msg
				logRequest(protocolLogger, msg)

			case msg.IsResponse():
				if request, ok := requests[msg.ID]; ok {
					delete(requests, msg.ID)

					if request != nil {
						logResponse(protocolLogger, request, msg)
					}
				}

			case msg.IsEvent():
				protocolLogger.WithFields(logrus.Fields{
					fieldType:   typeEvent,
					fieldMethod: msg.Method,
				}).Info(serialize(msg.Params))

			default:
				protocolLogger.WithFields(logrus.Fields{
					fieldType:   typeRequest,
					fieldMethod: msg.Method,
				}).Info("Could not understand message: " + msg.raw)
			}
		}
	}

	for sessionID := range sessions {
		_ = destroyLogger("session-" + sessionID)
	}
}

func checkVersion() (map[string]string, error) {
	res, err := http.Get("http://" + *flagRemote + "/json/version")
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
