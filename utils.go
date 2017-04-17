package main

import (
	"strings"
	"fmt"
	"encoding/json"
)

func center(message string, length int) string {
	padding := (length - len(message)) / 2

	if padding < 0 {
		return message
	} else {
		return strings.Repeat(" ", padding) + message + strings.Repeat(" ", length - len(message) - padding)
	}

}

func asString(value interface{}) string {
	if casted, ok := value.(string); ok {
		return casted
	}

	return fmt.Sprintf("%+v", value)
}

func serialize(value interface{}) string {

	if buff, err := json.Marshal(value); err == nil {
		if *flagEllipsis && len(buff) > ellipsisLength {
			return string(buff[:ellipsisLength]) + "..."
		}

		serialized := string(buff)

		if serialized == "null" {
			return "{}"
		}

		return serialized
	} else {
		return err.Error()
	}
}

func decodeMessage(bytes []byte) (*protocolMessage, error) {
	var msg protocolMessage

	if err := json.Unmarshal(bytes, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}