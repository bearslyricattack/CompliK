package lark

type LarkMessage struct {
	MsgType string      `json:"msg_type"`
	Card    interface{} `json:"card,omitempty"` // 卡片消息内容
}

type LarkResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
