package main

type ActionTrigger string

const (
	ActionTriggerHueTapDialButtonPress ActionTrigger = "hue tap dial button press"
	ActionTriggerHueLightSync          ActionTrigger = "hue light sync"
)

type GoveeAction string

const (
	GoveeActionTurnOn  GoveeAction = "turn on"
	GoveeActionTurnOff GoveeAction = "turn off"
)

type LightSyncValue string

const (
	LightSyncValueOnOff      LightSyncValue = "on-off"
	LightSyncValueBrightness LightSyncValue = "brightness"
	LightSyncValueColor      LightSyncValue = "color"
)

type ConfigurationAction struct {
	Trigger           ActionTrigger                    `json:"trigger"`
	DialName          string                           `json:"dial_name"`
	HueTapDialButtons []int                            `json:"hue_tap_dial_buttons"`
	GoveeActions      []ConfigurationActionGoveeAction `json:"govee_actions"`
	LightName         string                           `json:"light_name"`
}

type ConfigurationActionGoveeAction struct {
	Device          string         `json:"device"`
	Action          GoveeAction    `json:"action"`
	SyncValue       LightSyncValue `json:"sync_value"`
	BrightnessRange []int          `json:"brightness_range"`
}

type GoveeMessage struct {
	Device string
	Data   []byte
}
