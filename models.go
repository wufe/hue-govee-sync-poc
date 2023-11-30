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

type GoveeGenericResponse struct {
	Msg GoveeGenericResponseMsg `json:"msg"`
}

type GoveeGenericResponseMsg struct {
	Cmd  string      `json:"cmd"`
	Data interface{} `json:"data"`
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

type GoveeStatusRequest struct {
	Msg GoveeStatusRequestMsg `json:"msg"`
}

type GoveeStatusRequestMsg struct {
	Cmd  string                    `json:"cmd"`
	Data GoveeStatusRequestMsgData `json:"data"`
}

type GoveeStatusRequestMsgData struct {
}

type GoveeBrightnessRequest struct {
	Msg GoveeBrightnessRequestMsg `json:"msg"`
}

type GoveeBrightnessRequestMsg struct {
	Cmd  string                        `json:"cmd"`
	Data GoveeBrightnessRequestMsgData `json:"data"`
}

type GoveeBrightnessRequestMsgData struct {
	Value int `json:"value"`
}

type GoveeColorRequest struct {
	Msg GoveeColorRequestMsg `json:"msg"`
}

type GoveeColorRequestMsg struct {
	Cmd  string                   `json:"cmd"`
	Data GoveeColorRequestMsgData `json:"data"`
}

type GoveeColorRequestMsgData struct {
	Color  GoveeColorRequestMsgDataColor `json:"color"`
	Kelvin int                           `json:"colorTemInKelvin"`
}

type GoveeColorRequestMsgDataColor struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}
