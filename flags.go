package main

import (
	"flag"
)

var (
	flagListen         = flag.String("l", "localhost:9223", "listen address")
	flagRemote         = flag.String("r", "localhost:9222", "remote address")
	flagEllipsis       = flag.Bool("s", false, "shorten requests and responses")
	flagOnce           = flag.Bool("once", false, "debug single session")
	flagShowRequests   = flag.Bool("i", false, "include request frames as they are sent")
	flagDistributeLogs = flag.Bool("d", false, "write logs file per targetId")
	flagQuiet          = flag.Bool("q", false, "do not show logs on stdout")
	flagMicroseconds   = flag.Bool("m", false, "display time in microseconds")
	flagDelta          = flag.Bool("delta", false, "show delta time between log entries")
	flagDirLogs        = flag.String("log-dir", "logs", "logs directory")
	flagVersion        = flag.Bool("version", false, "display version information")
)
