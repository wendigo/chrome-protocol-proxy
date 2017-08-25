package main

import (
	"flag"
	"fmt"
	"strings"
)

type argumentList struct {
	name   string
	values []string
}

func (al *argumentList) String() string {
	return fmt.Sprintf("%s = %s", al.name, strings.Join(al.values, ", "))
}

func (al *argumentList) Set(value string) error {
	al.values = append(al.values, value)
	return nil
}

var filterInclude = &argumentList{name: "include", values: []string{}}
var filterExclude = &argumentList{name: "exclude", values: []string{}}

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
)

func init() {
	flag.Var(filterInclude, "include", "display only requests/responses/events matching pattern")
	flag.Var(filterExclude, "exclude", "exclude requests/responses/events matching pattern")
}
