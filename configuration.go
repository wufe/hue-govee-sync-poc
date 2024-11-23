package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/rs/zerolog/log"
	"github.com/sanity-io/litter"
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

	litter.Dump(configuration)

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
		if action.Trigger == ActionTriggerHueTapDialButtonPress {
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
