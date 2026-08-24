package main

import (
	"encoding/json"
	"strings"
)

func center(message string, length int) string {
	padding := (length - len(message)) / 2

	if padding < 0 {
		return message
	}

	return strings.Repeat(" ", padding) + message + strings.Repeat(" ", length-len(message)-padding)
}

func serialize(value interface{}) string {
	buff, ok := value.(json.RawMessage)
	if !ok {
		var err error
		if buff, err = json.Marshal(value); err != nil {
			return err.Error()
		}
	}

	if *flagEllipsis != 0 && len(buff) > *flagEllipsis {
		return string(buff[:*flagEllipsis]) + "..."
	}

	if len(buff) == 0 || string(buff) == "null" {
		return "{}"
	}

	return string(buff)
}

func decodeMessage(bytes []byte) (*protocolMessage, error) {
	var msg protocolMessage

	if err := json.Unmarshal(bytes, &msg); err != nil {
		return nil, err
	}

	msg.raw = string(bytes)
	return &msg, nil
}

func decodeProtocolMessage(message *protocolMessage) (*protocolMessage, error) {
	if !message.IsFlatten() && message.FromTargetDomain() {
		var params struct {
			Message string `json:"message"`
		}

		if err := json.Unmarshal(message.Params, &params); err != nil {
			return nil, err
		}

		return decodeMessage([]byte(params.Message))
	}

	return message, nil
}
