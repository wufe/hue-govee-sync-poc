package main

type GoveeScanRequest struct {
	Msg GoveeScanRequestMsg `json:"msg"`
}

type GoveeScanRequestMsg struct {
	Cmd  string                  `json:"cmd"`
	Data GoveeScanRequestMsgData `json:"data"`
}

type GoveeScanRequestMsgData struct {
	AccountTopic string `json:"account_topic"`
}

type GoveeScanResponse struct {
	Msg GoveeScanResponseMsg `json:"msg"`
}

type GoveeScanResponseMsg struct {
	Cmd  string                   `json:"cmd"`
	Data GoveeScanResponseMsgData `json:"data"`
}

type GoveeScanResponseMsgData struct {
	IP              string `json:"ip"`
	Device          string `json:"device"`
	SKU             string `json:"sku"`
	BleVersionHard  string `json:"bleVersionHard"`
	BleVersionSoft  string `json:"bleVersionSoft"`
	WifiVersionHard string `json:"wifiVersionHard"`
	WifiVersionSoft string `json:"wifiVersionSoft"`
}

type GoveeTurn struct {
	Msg GoveeTurnMsg `json:"msg"`
}

type GoveeTurnMsg struct {
	Cmd  string           `json:"cmd"`
	Data GoveeTurnMsgData `json:"data"`
}

type GoveeTurnMsgData struct {
	Value int `json:"value"`
}
