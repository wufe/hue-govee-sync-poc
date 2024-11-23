package main

type ActionTrigger string

const (
	ActionTriggerHueTapDialButtonPress ActionTrigger = "hue tap dial button press"
)

type GoveeAction string

const (
	GoveeActionTurnOn  GoveeAction = "turn on"
	GoveeActionTurnOff GoveeAction = "turn off"
)

type ConfigurationAction struct {
	Trigger           ActionTrigger                    `json:"trigger"`
	DialName          string                           `json:"dial_name"`
	HueTapDialButtons []int                            `json:"hue_tap_dial_buttons"`
	GoveeActions      []ConfigurationActionGoveeAction `json:"govee_actions"`
}

type ConfigurationActionGoveeAction struct {
	Device string      `json:"device"`
	Action GoveeAction `json:"action"`
}

type GoveeMessage struct {
	Device string
	Data   []byte
}
