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

func init() {
	flag.Var(filterInclude, "include", "display only requests/responses/events matching pattern")
	flag.Var(filterExclude, "exclude", "exclude requests/responses/events matching pattern")
}

func accept(values ...string) bool {

	value := strings.Join(values, "")

	for _, exclude := range filterExclude.values {
		if strings.Contains(value, exclude) {
			return false
		}
	}

	if len(filterInclude.values) == 0 {
		return true
	}

	for _, include := range filterInclude.values {
		if strings.Contains(value, include) {
			return true
		}
	}

	return false
}
