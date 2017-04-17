# chrome-protocol-proxy

```chrome-protocol-proxy``` is small reverse websocket proxy designed for ```chrome debugging protocol```. It's purpose is to capture messages written to and received from [Chrome Debugging Protocol](https://chromedevtools.github.io/debugger-protocol-viewer), coalesce requests with responses, unpack messages from [Target domain](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/) and provide easy to read, colored output. This tool is a fork of (and heavily inspired by) [chromedp-proxy](https://github.com/knq/chromedp/tree/master/cmd/chromedp-proxy).

## features
- colored output 🖖
- request-response coalescing,
- interprets [Target.sendMessageToTarget](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/#method-sendMessageToTarget) requests,
- interprets [Target.receivedMessageFromTarget](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/#event-receivedMessageFromTarget) responses and events.

## usage
```go get -u github.com/wendigo/chrome-protocol-proxy```

## configuration options
```
 Usage of chrome-protocol-proxy:
  -l string
    	listen address (default "localhost:9223")
  -log string
    	log file mask (default "logs/cdp-%s.log")
  -n	disable logging to file
  -once
    	debug single session
  -r string
    	remote address (default "localhost:9222")
  -s	shorten requests and responses
  ```
  
## demo
[![asciicast](https://asciinema.org/a/113947.png)](https://asciinema.org/a/113947?t=0:04&autoplay=1&speed=0.4)

## tips & tricks

When using [Headless Chrome](https://chromium.googlesource.com/chromium/src/+/lkgr/headless/README.md) navigate to [inspectable pages](http://localhost:9222/) and open inspector pane for url of your choosing. Then replace port in ```?ws=``` query param and point it to running ```chrome-protocol-proxy``` instance (default port is 9223). Now you're able to i see what Chrome Debugger is exactly doing. Enjoy!
