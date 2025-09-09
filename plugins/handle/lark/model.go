package lark

type LarkMessage struct {
	MsgType string `json:"msg_type"`
	Card    any    `json:"card,omitempty"`
}

type LarkResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
