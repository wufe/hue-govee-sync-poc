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

type TwinklyAction string

const (
	TwinklyActionTurnOn  TwinklyAction = "turn on"
	TwinklyActionTurnOff TwinklyAction = "turn off"
)

type LightSyncValue string

const (
	LightSyncValueOnOff      LightSyncValue = "on-off"
	LightSyncValueOn         LightSyncValue = "on"
	LightSyncValueOff        LightSyncValue = "off"
	LightSyncValueBrightness LightSyncValue = "brightness"
	LightSyncValueColor      LightSyncValue = "color"
)

type TwinklyMessage string

const (
	TwinklyMessageOn  TwinklyMessage = "on"
	TwinklyMessageOff TwinklyMessage = "off"
)

type ConfigurationAction struct {
	Trigger           ActionTrigger                    `json:"trigger"`
	DialName          string                           `json:"dial_name"`
	HueTapDialButtons []int                            `json:"hue_tap_dial_buttons"`
	GoveeActions      []ConfigurationActionGoveeAction `json:"govee_actions"`
	TwinklyActions    []ConfigurationTwinklyAction     `json:"twinkly_actions"`
	LightName         string                           `json:"light_name"`
}

type ConfigurationActionGoveeAction struct {
	Device          string         `json:"device"`
	Action          GoveeAction    `json:"action"`
	SyncValue       LightSyncValue `json:"sync_value"`
	BrightnessRange []int          `json:"brightness_range"`
}

type ConfigurationTwinklyAction struct {
	// No "Device", because only one device is supported for now
	SyncValue LightSyncValue `json:"sync_value"` // Only "on-off" supported for twinkly
	Action    TwinklyAction  `json:"action"`
}

type GoveeMessage struct {
	Device string
	Data   []byte
}
