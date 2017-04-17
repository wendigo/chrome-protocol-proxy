# chrome-protocol-proxy

```chrome-protocol-proxy``` is small proxy designed for ```chrome debugging protocol```. It's purpose is to capture messages written to and received from [Chrome Debugging Protocol](https://chromedevtools.github.io/debugger-protocol-viewer), assemble requests with responses, unpack messages from [Target domain](https://chromedevtools.github.io/debugger-protocol-viewer/tot/Target/) and provide easy to read, styles output. This tool is a fork (and heavily inspired by) of [chromedp-proxy](https://github.com/knq/chromedp/tree/master/cmd/chromedp-proxy).

## usage
```go get -u github.com/wendigo/chrome-protocol-proxy```

```chrome-protocol-proxy --help```

## configuration options
```
  -l string
    	listen address (default "localhost:9223")
  -log string
    	log file mask (default "logs/cdp-%s.log")
  -n	disable logging to file
  -r string
    	remote address (default "localhost:9222")
  -s	shorten requests and responses
  ```
  
## demo
[![asciicast](https://asciinema.org/a/113947.png)](https://asciinema.org/a/113947?t=0:05)

## tips & tricks

When using [Headless Chrome](https://chromium.googlesource.com/chromium/src/+/lkgr/headless/README.md) navigate to [inspectable pages](http://localhost:9222/) and open inspector pane for url of your choosing. Then replace port in ```?ws=``` query param and point it to running ```chrome-protocol-proxy``` instance (default port is 9223). Now you're able to i see what Chrome Debugger is exactly doing. Enjoy!
