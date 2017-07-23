package main

import "fmt"

type protocolMessage struct {
	Id     uint64                 `json:"id"`
	Result map[string]interface{} `json:"result"`
	Error  struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type targetedProtocolMessage struct {
	TargetId string `json:"targetId"`
	SessionId string `json:"sessionId"`
	Message  string `json:"message"`
}

func (t *targetedProtocolMessage) ProtocolMessage() (*protocolMessage, error) {
	return decodeMessage([]byte(t.Message))
}

func (p *protocolMessage) String() string {
	return fmt.Sprintf(
		"protocolMessage{id=%d, method=%s, result=%+v, error=%+v, params=%+v}",
		p.Id,
		p.Method,
		p.Result,
		p.Error,
		p.Params,
	)
}

func (p *protocolMessage) IsError() bool {
	return p.Error.Code != 0
}

func (p *protocolMessage) IsResponse() bool {
	return p.Method == "" && p.Id > 0
}

func (p *protocolMessage) IsRequest() bool {
	return p.Method != "" && p.Id > 0
}

func (p *protocolMessage) IsEvent() bool {
	return !(p.IsRequest() || p.IsResponse())
}

func (p *protocolMessage) InTarget() bool {
	return p.Method == "Target.sendMessageToTarget" || p.Method == "Target.receivedMessageFromTarget"
}

func (p *protocolMessage) TargetId() string {
	if (p.InTarget()) {
		if val, ok := p.Params["sessionId"]; ok {
			return val.(string)
		}

		if val, ok := p.Params["targetId"]; ok {
			return val.(string)
		}
	}

	return ""
}
