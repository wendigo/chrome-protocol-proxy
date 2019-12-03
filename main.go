// This software is direct fork of https://github.com/knq/chromedp/tree/master/cmd/chromedp-proxy
// with couple of features added
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"

	"errors"

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

			// connect incoming websocket
			logger.Infof("upgrading connection on %s...", req.RemoteAddr)
			in, err := wsUpgrader.Upgrade(res, req, nil)
			if err != nil {
				logger.Errorf("could not upgrade websocket from %s: %v", req.RemoteAddr, err)
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

	log.Printf("Listening for DevTools connections on: %s", *flagListen)

	log.Fatal(http.ListenAndServe(*flagListen, mux))
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
	targetRequests := make(map[uint64]*protocolMessage)
	sessions := make(map[string]interface{}, 0)

loop:
	for {
		select {
		case msg, ok := <-stream:
			if !ok {
				for sessionId := range sessions {
					_ = destroyLogger(fmt.Sprintf("session-%s", sessionId))
				}
				break loop
			}

			if msg.HasSessionId() {
				var targetLogger *logrus.Entry

				if *flagDistributeLogs {
					logger, err := createLogger(fmt.Sprintf("session-%s", msg.TargetID()))
					sessions[msg.TargetID()] = interface{}(nil)

					if err != nil {
						panic(fmt.Sprintf("could not create logger: %v", err))
					}

					targetLogger = logger.WithFields(logrus.Fields{
						fieldLevel:    levelTarget,
						fieldTargetID: msg.TargetID(),
					})

				} else {
					targetLogger = logger.WithFields(logrus.Fields{
						fieldLevel:    levelTarget,
						fieldTargetID: msg.TargetID(),
					})
				}

				if msg.IsRequest() {
					requests[msg.ID] = nil

					if protocolMessage, err := decodeProtocolMessage(msg); err == nil {
						targetRequests[protocolMessage.ID] = protocolMessage

						if *flagShowRequests {
							targetLogger.WithFields(logrus.Fields{
								fieldType:   typeRequest,
								fieldMethod: protocolMessage.Method,
							}).Info(serialize(protocolMessage.Params))
						}

					} else {
						logger.WithFields(logrus.Fields{
							fieldLevel: levelConnection,
						}).Errorf("Could not deserialize message: %+v", err)
					}
				}

				if msg.IsEvent() {
					if protocolMessage, err := decodeProtocolMessage(msg); err == nil {
						if protocolMessage.IsEvent() {
							targetLogger.WithFields(logrus.Fields{
								fieldType:   typeEvent,
								fieldMethod: protocolMessage.Method,
							}).Info(serialize(protocolMessage.Params))
						}

						if protocolMessage.IsResponse() {
							var logMessage string
							var logType int
							var logRequest string
							var logMethod string

							if protocolMessage.IsError() {
								logMessage = serialize(protocolMessage.Error)
								logType = typeRequestResponseError
							} else {
								logMessage = serialize(protocolMessage.Result)
								logType = typeRequestResponse
							}

							if request, ok := targetRequests[protocolMessage.ID]; ok && request != nil {
								delete(targetRequests, protocolMessage.ID)
								logRequest = serialize(request.Params)
								logMethod = request.Method

							} else {
								logRequest = errorColor("could not find request with id: %d", protocolMessage.ID)
							}

							targetLogger.WithFields(logrus.Fields{
								fieldType:    logType,
								fieldMethod:  logMethod,
								fieldRequest: logRequest,
							}).Info(logMessage)
						}
					} else {
						logger.WithFields(logrus.Fields{
							fieldLevel: levelConnection,
						}).Errorf("Could not deserialize message: %+v", err)
					}
				}

			} else {
				protocolLogger := logger.WithFields(logrus.Fields{
					fieldLevel:    levelProtocol,
					fieldTargetID: protocolTargetID,
				})

				if msg.IsRequest() {
					requests[msg.ID] = msg

					if *flagShowRequests {
						protocolLogger.WithFields(logrus.Fields{
							fieldType:   typeRequest,
							fieldMethod: msg.Method,
						}).Info(serialize(msg.Params))
					}
				}

				if msg.IsResponse() {

					var logMessage string
					var logType int
					var logRequest string
					var logMethod string

					if msg.IsError() {
						logMessage = serialize(msg.Error)
						logType = typeRequestResponseError
					} else {
						logMessage = serialize(msg.Result)
						logType = typeRequestResponse
					}

					if request, ok := requests[msg.ID]; ok && request != nil {
						logRequest = serialize(request.Params)
						logMethod = request.Method

						delete(requests, msg.ID)

						protocolLogger.WithFields(logrus.Fields{
							fieldType:    logType,
							fieldMethod:  logMethod,
							fieldRequest: logRequest,
						}).Info(logMessage)
					}
				}

				if msg.IsEvent() {
					protocolLogger.WithFields(logrus.Fields{
						fieldType:   typeEvent,
						fieldMethod: msg.Method,
					}).Info(serialize(msg.Params))
				}
			}
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
