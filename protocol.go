package main

import (
	"encoding/json"
	"fmt"
)

type protocolMessage struct {
	/**
	The raw message as string.
	*/
	raw    string
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	SessionId string          `json:"sessionId"`
}

func (p *protocolMessage) String() string {
	return fmt.Sprintf(
		"protocolMessage{id=%d, method=%s, sessionId=%s, result=%s, error=%+v, params=%s}",
		p.ID,
		p.Method,
		p.SessionId,
		p.Result,
		p.Error,
		p.Params,
	)
}

func (p *protocolMessage) IsError() bool {
	return p.Error.Code != 0
}

func (p *protocolMessage) IsResponse() bool {
	return p.ID > 0 && (len(p.Result) > 0 || p.IsError())
}

func (p *protocolMessage) IsRequest() bool {
	return p.Method != "" && p.ID > 0
}

func (p *protocolMessage) IsEvent() bool {
	return !(p.IsRequest() || p.IsResponse())
}

func (p *protocolMessage) FromTargetDomain() bool {
	return p.Method == "Target.sendMessageToTarget" || p.Method == "Target.receivedMessageFromTarget"
}

func (p *protocolMessage) HasSessionId() bool {
	return p.FromTargetDomain() || p.IsFlatten()
}

func (p *protocolMessage) IsFlatten() bool {
	return p.SessionId != ""
}

func (p *protocolMessage) TargetID() string {
	if p.SessionId != "" {
		return p.SessionId
	}

	if p.FromTargetDomain() {
		var params struct {
			SessionID string `json:"sessionId"`
		}

		if json.Unmarshal(p.Params, &params) == nil {
			return params.SessionID
		}
	}

	return ""
}
