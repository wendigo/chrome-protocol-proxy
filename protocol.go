package main

import "fmt"

type protocolMessage struct {
	ID     uint64                 `json:"id"`
	Result map[string]interface{} `json:"result"`
	Error  struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
	Method    string                 `json:"method"`
	Params    map[string]interface{} `json:"params"`
	SessionId string                 `json:"sessionId"`
}

func (p *protocolMessage) String() string {
	return fmt.Sprintf(
		"protocolMessage{id=%d, method=%s, sessionId=%s, result=%+v, error=%+v, params=%+v}",
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
	return p.Method == "" && p.ID > 0
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
		if val, ok := p.Params["sessionId"]; ok {
			return val.(string)
		}
	}

	return ""
}
