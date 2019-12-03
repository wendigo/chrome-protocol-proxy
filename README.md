# chrome-protocol-proxy

```chrome-protocol-proxy``` is small, reverse proxy designed for working with [Chrome's DevTools protocol](https://github.com/ChromeDevTools/devtools-protocol). It captures all commands sent to and events received from Chrome, coalesce requests with responses, unpack messages from [Target domain](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/) and provide easy to read, colored output. This tool is a fork of (and heavily inspired by) [chromedp-proxy](https://github.com/knq/chromedp/tree/master/cmd/chromedp-proxy).

![chrome-protocol-proxy screenshot](https://pbs.twimg.com/media/C9nifD2WsAEkl4s.jpg:large)

# Installation

## Via homebrew

```brew install wendigo/tap/chrome-protocol-proxy```

## Via go get

```go get -u github.com/wendigo/chrome-protocol-proxy```

## Via docker

```docker run -t -i -p 9222:9222 wendigo/chrome-protocol-proxy:latest```

### Validate installation

Head to [http://localhost:9222](http://localhost:9222).

# Features
- colored output,
- protocol frames filtering,🖖
- request-response coalescing,
- interprets [Target.sendMessageToTarget](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/#method-sendMessageToTarget) requests,
- interprets [Target.receivedMessageFromTarget](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/#event-receivedMessageFromTarget) responses and events with [sessionId](https://chromium.googlesource.com/chromium/src/+/237f82767da3bbdcd8d6ad3fa4449ef6a3fe8bd3),
- understands flatted sessions ([crbug.com/991325](crbug.com/991325))
- calculates and displays time delta between consecutive frames,
- writes logs and splits them based on connection id and target/session id.

# Configuration flags
```
-d	write logs file per targetId
-delta
   show delta time between log entries
-exclude value
   exclude requests/responses/events matching pattern (default exclude = )
-i	include request frames as they are sent
-include value
   display only requests/responses/events matching pattern (default include = )
-l string
   listen address (default "localhost:9223")
-log-dir string
   logs directory (default "logs")
-m	display time in microseconds
-once
   debug single session
-q	do not show logs on stdout
-r string
   remote address (default "localhost:9222")
-s	shorten requests and responses
-version
   display version information
  ```

# Demo
[![asciicast](https://asciinema.org/a/113947.png)](https://asciinema.org/a/113947?t=0:04&autoplay=1&speed=0.4)
