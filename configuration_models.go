package main

import "fmt"

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

type SwitchbotAction string

const (
	SwitchbotActionTurnOn  SwitchbotAction = "turn on"
	SwitchbotActionTurnOff SwitchbotAction = "turn off"
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
	Trigger           ActionTrigger                        `json:"trigger"`
	DialName          string                               `json:"dial_name"`
	HueTapDialButtons []int                                `json:"hue_tap_dial_buttons"`
	GoveeActions      []ConfigurationActionGoveeAction     `json:"govee_actions"`
	TwinklyActions    []ConfigurationTwinklyAction         `json:"twinkly_actions"`
	SwitchbotActions  []ConfigurationActionSwitchbotAction `json:"switchbot_actions"`
	LightName         string                               `json:"light_name"`
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

type ConfigurationActionSwitchbotAction struct {
	Device          string          `json:"device"` // Friendly name previously set in map key of <conf_root>->Switchbot
	Action          SwitchbotAction `json:"action"`
	SyncValue       LightSyncValue  `json:"sync_value"`
	BrightnessRange []int           `json:"brightness_range"`
}

type GoveeMessage struct {
	Device string
	Data   []byte
}

type SwitchbotDeviceConfiguration struct {
	DeviceID      string                              `json:"device_id"`
	Authorization SwitchbotAuthorizationConfiguration `json:"authorization"`
}

type SwitchbotAuthorizationConfiguration struct {
	Token string `json:"token"`
}

type SwitchbotMessage struct {
	Device      string `json:"device"`
	Command     string `json:"command"`
	Parameter   string `json:"parameter"`
	CommandType string `json:"commandType"`
}

func (m SwitchbotMessage) IsEmpty() bool {
	return m.Command == ""
}

func NewSwitchbotMessageForDevice(device string) SwitchbotMessage {
	return SwitchbotMessage{
		Device: device,
	}
}

func (m SwitchbotMessage) TurnOn() SwitchbotMessage {
	m.Command = "turnOn"
	m.Parameter = "default"
	m.CommandType = "command"
	return m
}

func (m SwitchbotMessage) TurnOff() SwitchbotMessage {
	m.Command = "turnOff"
	m.Parameter = "default"
	m.CommandType = "command"
	return m
}

func (m SwitchbotMessage) SetBrightness(brightness int) SwitchbotMessage {
	m.Command = "setBrightness"
	m.Parameter = fmt.Sprintf("%d", brightness)
	m.CommandType = "command"
	return m
}
