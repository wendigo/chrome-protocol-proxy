package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func center(message string, length int) string {
	padding := (length - len(message)) / 2

	if padding < 0 {
		return message
	}

	return strings.Repeat(" ", padding) + message + strings.Repeat(" ", length-len(message)-padding)
}

func asString(value interface{}) string {
	if casted, ok := value.(string); ok {
		return casted
	}

	return fmt.Sprintf("%+v", value)
}

func serialize(value interface{}) string {

	buff, err := json.Marshal(value)
	if err == nil {
		if *flagEllipsis != 0 && len(buff) > *flagEllipsis {
			return string(buff[:*flagEllipsis]) + "..."
		}

		serialized := string(buff)

		if serialized == "null" {
			return "{}"
		}

		return serialized
	}

	return err.Error()
}

func decodeMessage(bytes []byte) (*protocolMessage, error) {
	var msg protocolMessage

	if err := json.Unmarshal(bytes, &msg); err != nil {

		return nil, err
	}

	msg.raw = string(bytes[:])
	return &msg, nil
}

func decodeProtocolMessage(message *protocolMessage) (*protocolMessage, error) {
	if message.IsFlatten() {
		return message, nil
	}

	if message.FromTargetDomain() {
		return decodeMessage([]byte(asString(message.Params["message"])))
	}

	return message, nil
}
