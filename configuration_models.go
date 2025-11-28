package main

import "fmt"

type ActionTrigger string

const (
	ActionTriggerHueTapDialButtonPress ActionTrigger = "hue tap dial button press"
	ActionTriggerHueLightSync          ActionTrigger = "hue light sync"
	ActionTriggerPresenceSensor        ActionTrigger = "presence sensor"
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

type WledAction string

const (
	WledActionTurnOn             WledAction = "turn on"
	WledActionTurnOff            WledAction = "turn off"
	WledActionSetBrightness      WledAction = "set brightness"
	WledActionIncreaseBrightness WledAction = "increase brightness"
	WledActionDecreaseBrightness WledAction = "decrease brightness"
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
	Trigger            ActionTrigger                        `json:"trigger"`
	DialName           string                               `json:"dial_name"`
	PresenceSensorName string                               `json:"presence_sensor_name"`
	HueTapDialButtons  []int                                `json:"hue_tap_dial_buttons"`
	GoveeActions       []ConfigurationActionGoveeAction     `json:"govee_actions"`
	TwinklyActions     []ConfigurationTwinklyAction         `json:"twinkly_actions"`
	SwitchbotActions   []ConfigurationActionSwitchbotAction `json:"switchbot_actions"`
	WledActions        []ConfigurationActionWledAction      `json:"wled_actions"`
	LightName          string                               `json:"light_name"`
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

type ConfigurationActionWledAction struct {
	Device          string         `json:"device"`
	Action          WledAction     `json:"action"`
	SyncValue       LightSyncValue `json:"sync_value"`
	BrightnessRange []int          `json:"brightness_range"`
	Value           any            `json:"value"` // like for example "10" or for brightness increase, or "10" to set brightness exactly to 10
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

type WledDeviceConfiguration struct {
	Device string `json:"device"`
	IP     string `json:"ip"`
}

type WledMessage struct {
	Device string `json:"device"`
	Body   []byte `json:"body"`
}

func NewWledMessageForDevice(device string) WledMessage {
	return WledMessage{
		Device: device,
	}
}

func (m WledMessage) IsEmpty() bool {
	return m.Body == nil
}

func (m WledMessage) TurnOn() WledMessage {
	m.Body = []byte(`{"on":true}`)
	return m
}

func (m WledMessage) TurnOff() WledMessage {
	m.Body = []byte(`{"on":false}`)
	return m
}

// SetBrightness sets the brightness (0-100) in the WLED message
func (m WledMessage) SetBrightness(brightness int) WledMessage {
	brightness = mapBrightness(brightness, []int{0, 100}, []int{0, 255})
	m.Body = []byte(fmt.Sprintf(`{"bri":%d}`, brightness))
	return m
}
