package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/rs/zerolog/log"
)

type Configuration struct {
	BridgeIP string                `json:"bridge_ip"`
	AppName  string                `json:"app_name"`
	Actions  []ConfigurationAction `json:"actions"`
}

func NewConfiguration() Configuration {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("error getting working directory: %w", err))
	}

	configurationFilePath := path.Join(wd, "configuration.json")
	absoluteConfigurationFilePath, err := filepath.Abs(configurationFilePath)
	if err != nil {
		panic(fmt.Errorf("error getting absolute path for configuration file: %w", err))
	}

	if _, err := os.Stat(absoluteConfigurationFilePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			panic(fmt.Errorf("configuration file not found: %w", err))
		}
		panic(fmt.Errorf("error checking configuration file: %w", err))
	}

	rawConfiguration, err := os.ReadFile(absoluteConfigurationFilePath)
	if err != nil {
		panic(fmt.Errorf("error reading configuration file: %w", err))
	}

	var configuration Configuration
	if err := json.Unmarshal(rawConfiguration, &configuration); err != nil {
		panic(fmt.Errorf("error unmarshalling configuration: %w", err))
	}

	return configuration
}

func (c *Configuration) GetRequiredGoveeDevices() []string {
	var devices []string
	for _, action := range c.Actions {
		for _, goveeAction := range action.GoveeActions {
			devices = append(devices, goveeAction.Device)
		}
	}
	return devices
}

func (c *Configuration) GetRequiredHueDials() []string {
	var dials []string
	for _, action := range c.Actions {
		if action.DialName != "" {
			dials = append(dials, action.DialName)
		}
	}
	return dials
}

func (c *Configuration) GetMessagesToDispatchOnHueTapDialButtonPressed(dialName string, buttonPressed int) []GoveeMessage {
	var messages []GoveeMessage
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueTapDialButtonPress && action.DialName == dialName {
			if slices.Contains(action.HueTapDialButtons, buttonPressed) {
				for _, goveeAction := range action.GoveeActions {
					var message []byte
					switch goveeAction.Action {
					case GoveeActionTurnOn:
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 1,
								},
							},
						})
					case GoveeActionTurnOff:
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 0,
								},
							},
						})
					default:
						err := fmt.Errorf("unknown Govee action: %s", goveeAction.Action)
						log.Err(err).Msgf("Error creating Govee message: %s", err)
						continue
					}
					messages = append(messages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
				}
			}

		}
	}
	return messages
}

func (c *Configuration) IsLightRequired(lightName string) bool {
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			return true
		}
	}

	return false
}

func (c *Configuration) GetMessagesToDispatchOnHueLightOnOffChange(lightName string, on bool) []GoveeMessage {
	var messages []GoveeMessage
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			for _, goveeAction := range action.GoveeActions {
				var message []byte
				switch goveeAction.SyncValue {
				case LightSyncValueOnOff:
					if on {
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 1,
								},
							},
						})
					} else {
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 0,
								},
							},
						})
					}
				}
				if message != nil {
					messages = append(messages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
				}
			}
		}
	}
	return messages
}

func (c *Configuration) GetMessagesToDispatchOnHueLightBrightnessChange(lightName string, brightness int) []GoveeMessage {
	var messages []GoveeMessage
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			for _, goveeAction := range action.GoveeActions {
				var message []byte
				switch goveeAction.SyncValue {
				case LightSyncValueBrightness:
					if len(goveeAction.BrightnessRange) == 2 {
						brightnessRangeDelta := goveeAction.BrightnessRange[1] - goveeAction.BrightnessRange[0]
						brightnessPercentageDelta := float64(brightnessRangeDelta) * (float64(brightness) / 100)
						brightness = int(float64(goveeAction.BrightnessRange[0]) + brightnessPercentageDelta)
						brightness = int(math.Min(math.Max(float64(brightness), 0), 100))
					}
					message = mustMarshal(GoveeBrightnessRequest{
						Msg: GoveeBrightnessRequestMsg{
							Cmd: "brightness",
							Data: GoveeBrightnessRequestMsgData{
								Value: brightness,
							},
						},
					})
				}
				if message != nil {
					messages = append(messages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
				}
			}
		}
	}
	return messages
}

func (c *Configuration) GetMessagesToDispatchOnHueLightColorChange(lightName string, r, g, b uint8) []GoveeMessage {
	var messages []GoveeMessage
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			for _, goveeAction := range action.GoveeActions {
				var message []byte
				switch goveeAction.SyncValue {
				case LightSyncValueColor:
					message = mustMarshal(GoveeColorRequest{
						Msg: GoveeColorRequestMsg{
							Cmd: "colorwc",
							Data: GoveeColorRequestMsgData{
								Color: GoveeColorRequestMsgDataColor{
									R: int(r),
									G: int(g),
									B: int(b),
								},
							},
						},
					})
				}
				if message != nil {
					messages = append(messages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
				}
			}
		}
	}
	return messages
}
