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
	"time"

	"github.com/bluele/gcache"
	"github.com/rs/zerolog/log"
)

type Configuration struct {
	BridgeIP  string                                  `json:"bridge_ip"`
	AppName   string                                  `json:"app_name"`
	Actions   []ConfigurationAction                   `json:"actions"`
	Govee     map[string]GoveeDeviceConfiguration     `json:"govee"`
	Hue       HueConfiguration                        `json:"hue"`
	Switchbot map[string]SwitchbotDeviceConfiguration `json:"switchbot"`
	Wled      map[string]WledDeviceConfiguration      `json:"wled"`
	Twinkly   map[string]TwinklyDeviceConfiguration   `json:"twinkly"`

	presenceSensorActionsCache gcache.Cache
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

	configuration.presenceSensorActionsCache = gcache.New(0).
		Expiration(60 * time.Second).
		LoaderFunc(func(i any) (any, error) {
			var foundAction *ConfigurationAction
			for _, action := range configuration.Actions {
				if action.Trigger == ActionTriggerPresenceSensor {
					if action.PresenceSensorName == i.(string) {
						foundAction = &action
						return foundAction, nil
					}
				}
			}
			return foundAction, nil
		}).
		Build()

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

func (c *Configuration) GetAllGoveeDeviceAliases() []string {
	var aliases []string
	for alias := range c.Govee {
		aliases = append(aliases, alias)
	}
	return aliases
}

func (c *Configuration) GetGoveeDeviceAliasByMAC(mac string) (string, bool) {
	for alias, deviceConfig := range c.Govee {
		if deviceConfig.MAC == mac {
			return alias, true
		}
	}
	return "", false
}

func (c *Configuration) GetAllSwitchbotDeviceAliases() []string {
	var aliases []string
	for alias := range c.Switchbot {
		aliases = append(aliases, alias)
	}
	return aliases
}

func (c *Configuration) GetAllWledDeviceAliases() []string {
	var aliases []string
	for alias := range c.Wled {
		aliases = append(aliases, alias)
	}
	return aliases
}

func (c *Configuration) GetAllTwinklyDeviceAliases() []string {
	var aliases []string
	for alias := range c.Twinkly {
		aliases = append(aliases, alias)
	}
	return aliases
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

func (c *Configuration) GetMessagesToDispatchOnHueTapDialButtonPressed(
	dialName string,
	buttonPressed int,
	wledBrightnessRetriever BrightnessRetriever,
) ([]GoveeMessage, []TwinklyMessage, []SwitchbotMessage, []WledMessage) {
	var goveeMessages []GoveeMessage
	var twinklyMessages []TwinklyMessage
	var switchbotMessages []SwitchbotMessage
	var wledMessages []WledMessage
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
						Status.SetOn(goveeAction.Device, true)
					case GoveeActionTurnOff:
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 0,
								},
							},
						})
						Status.SetOn(goveeAction.Device, false)
					default:
						err := fmt.Errorf("unknown Govee action: %s", goveeAction.Action)
						log.Err(err).Msgf("Error creating Govee message: %s", err)
						continue
					}
					goveeMessages = append(goveeMessages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
				}
				for _, twinklyAction := range action.TwinklyActions {
					var message TwinklyMessage
					switch twinklyAction.Action {
					case TwinklyActionTurnOn:
						message = TwinklyMessageOn
						Status.SetOn("Twinkly Device", true) // TODO: Add support to multiple devices
					case TwinklyActionTurnOff:
						message = TwinklyMessageOff
						Status.SetOn("Twinkly Device", false) // TODO: Add support to multiple devices
					default:
						err := fmt.Errorf("unknown Twinkly action: %s", twinklyAction.Action)
						log.Err(err).Msgf("Error creating Twinkly message: %s", err)
						continue
					}
					twinklyMessages = append(twinklyMessages, message)
				}
				for _, switchbotAction := range action.SwitchbotActions {
					message := NewSwitchbotMessageForDevice(switchbotAction.Device)
					switch switchbotAction.Action {
					case SwitchbotActionTurnOn:
						message = message.TurnOn()
						Status.SetOn(switchbotAction.Device, true)
					case SwitchbotActionTurnOff:
						message = message.TurnOff()
						Status.SetOn(switchbotAction.Device, false)
					default:
						err := fmt.Errorf("unknown Switchbot action: %s", switchbotAction.Action)
						log.Err(err).Msgf("Error creating Switchbot message: %s", err)
						continue
					}
					if !message.IsEmpty() {
						switchbotMessages = append(switchbotMessages, message)
					}
				}
				for _, wledAction := range action.WledActions {
					message := NewWledMessageForDevice(wledAction.Device)
					switch wledAction.Action {
					case WledActionTurnOn:
						message = message.TurnOn()
						Status.SetOn(wledAction.Device, true)
					case WledActionTurnOff:
						message = message.TurnOff()
						Status.SetOn(wledAction.Device, false)
					case WledActionSetBrightness:
						floatVal, ok := wledAction.Value.(float64)
						if !ok {
							floatVal = 50
						}
						intVal := int(floatVal)
						message = message.SetBrightness(intVal)
						Brightness.SetForDevice(wledAction.Device, intVal)
						Status.SetBrightness(wledAction.Device, intVal)
					case WledActionIncreaseBrightness:
						floatVal, ok := wledAction.Value.(float64)
						if !ok {
							floatVal = 10
						}
						intVal := int(floatVal)
						currentBrightness := Brightness.GetDeviceBrightness(wledAction.Device, WithOnMissingBrightness(wledBrightnessRetriever.GetDeviceBrightness))
						newBrightness := int(math.Min(float64(currentBrightness+intVal), 100))
						message = message.SetBrightness(newBrightness)
						Brightness.SetForDevice(wledAction.Device, newBrightness)
						Status.SetBrightness(wledAction.Device, newBrightness)
					case WledActionDecreaseBrightness:
						floatVal, ok := wledAction.Value.(float64)
						if !ok {
							floatVal = 10
						}
						intVal := int(floatVal)
						currentBrightness := Brightness.GetDeviceBrightness(wledAction.Device, WithOnMissingBrightness(wledBrightnessRetriever.GetDeviceBrightness))
						newBrightness := int(math.Max(float64(currentBrightness-intVal), 0))
						message = message.SetBrightness(newBrightness)
						Brightness.SetForDevice(wledAction.Device, newBrightness)
						Status.SetBrightness(wledAction.Device, newBrightness)
					default:
						err := fmt.Errorf("unknown WLED action: %s", wledAction.Action)
						log.Err(err).Msgf("Error creating WLED message: %s", err)
						continue
					}
					if !message.IsEmpty() {
						log.Info().Msgf("Adding WLED message for device %s, action %s", wledAction.Device, wledAction.Action)
						wledMessages = append(wledMessages, message)
					}
				}
			}

		}
	}
	return goveeMessages, twinklyMessages, switchbotMessages, wledMessages
}

func (c *Configuration) IsLightRequired(lightName string) bool {
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			return true
		}
	}

	return false
}

func (c *Configuration) GetMessagesToDispatchOnHueLightOnOffChange(lightName string, on bool) ([]GoveeMessage, []TwinklyMessage, []SwitchbotMessage, []WledMessage) {
	var goveeMessages []GoveeMessage
	var twinklyMessages []TwinklyMessage
	var switchbotMessages []SwitchbotMessage
	var wledMessages []WledMessage
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			for _, goveeAction := range action.GoveeActions {
				var message []byte
				switch {
				case goveeAction.SyncValue == LightSyncValueOnOff:
					if on {
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 1,
								},
							},
						})
						Status.SetOn(goveeAction.Device, true)
					} else {
						message = mustMarshal(GoveeTurn{
							Msg: GoveeTurnMsg{
								Cmd: "turn",
								Data: GoveeTurnMsgData{
									Value: 0,
								},
							},
						})
						Status.SetOn(goveeAction.Device, false)
					}
				case goveeAction.SyncValue == LightSyncValueOn && on:
					message = mustMarshal(GoveeTurn{
						Msg: GoveeTurnMsg{
							Cmd: "turn",
							Data: GoveeTurnMsgData{
								Value: 1,
							},
						},
					})
					Status.SetOn(goveeAction.Device, true)
				case goveeAction.SyncValue == LightSyncValueOff && !on:
					message = mustMarshal(GoveeTurn{
						Msg: GoveeTurnMsg{
							Cmd: "turn",
							Data: GoveeTurnMsgData{
								Value: 0,
							},
						},
					})
					Status.SetOn(goveeAction.Device, false)
				}
				if message != nil {
					goveeMessages = append(goveeMessages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
				}
			}
			for _, twinklyAction := range action.TwinklyActions {
				var message TwinklyMessage
				switch twinklyAction.SyncValue {
				case LightSyncValueOnOff:
					if on {
						message = TwinklyMessageOn
					} else {
						message = TwinklyMessageOff
					}
				case LightSyncValueOn:
					if on {
						message = TwinklyMessageOn
					}
				case LightSyncValueOff:
					if !on {
						message = TwinklyMessageOff
					}
				}
				if message != "" {
					if message == TwinklyMessageOn {
						Status.SetOn("Twinkly Device", true) // TODO: Add support to multiple devices
					} else {
						Status.SetOn("Twinkly Device", false) // TODO: Add support to multiple devices
					}
					twinklyMessages = append(twinklyMessages, message)
				}
			}
			for _, switchbotAction := range action.SwitchbotActions {
				message := NewSwitchbotMessageForDevice(switchbotAction.Device)
				switch switchbotAction.SyncValue {
				case LightSyncValueOnOff:
					if on {
						message = message.TurnOn()
						Status.SetOn(switchbotAction.Device, true)
					} else {
						message = message.TurnOff()
						Status.SetOn(switchbotAction.Device, false)
					}
				case LightSyncValueOn:
					if on {
						message = message.TurnOn()
						Status.SetOn(switchbotAction.Device, true)
					}
				case LightSyncValueOff:
					if !on {
						message = message.TurnOff()
						Status.SetOn(switchbotAction.Device, false)
					}
				}
				if !message.IsEmpty() {
					switchbotMessages = append(switchbotMessages, message)
				}
			}
			for _, wledAction := range action.WledActions {
				message := NewWledMessageForDevice(wledAction.Device)
				switch wledAction.SyncValue {
				case LightSyncValueOnOff:
					if on {
						message = message.TurnOn()
						Status.SetOn(wledAction.Device, true)
					} else {
						message = message.TurnOff()
						Status.SetOn(wledAction.Device, false)
					}
				case LightSyncValueOn:
					if on {
						message = message.TurnOn()
						Status.SetOn(wledAction.Device, true)
					}
				case LightSyncValueOff:
					if !on {
						message = message.TurnOff()
						Status.SetOn(wledAction.Device, false)
					}
				}
				if !message.IsEmpty() {
					wledMessages = append(wledMessages, message)
				}
			}
		}
	}
	return goveeMessages, twinklyMessages, switchbotMessages, wledMessages
}

func (c *Configuration) GetMessagesToDispatchOnHueLightBrightnessChange(lightName string, brightness int) ([]GoveeMessage, []SwitchbotMessage, []WledMessage) {
	var goveeMessages []GoveeMessage
	var switchbotMessages []SwitchbotMessage
	var wledMessages []WledMessage
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerHueLightSync && action.LightName == lightName {
			for _, goveeAction := range action.GoveeActions {
				var message []byte
				brightnessValueChanged := -1
				switch goveeAction.SyncValue {
				case LightSyncValueBrightness:
					brightnessToSend := brightness
					if len(goveeAction.BrightnessRange) == 2 {
						// brightnessRangeDelta := goveeAction.BrightnessRange[1] - goveeAction.BrightnessRange[0]
						// brightnessPercentageDelta := float64(brightnessRangeDelta) * (float64(brightness) / 100)
						// brightnessToSend = int(float64(goveeAction.BrightnessRange[0]) + brightnessPercentageDelta)
						// brightnessToSend = int(math.Min(math.Max(float64(brightnessToSend), 0), 100))
						brightnessToSend = getAdjustedBrightnessByRange(brightness, goveeAction.BrightnessRange)
					}
					brightnessValueChanged = brightnessToSend
					message = mustMarshal(GoveeBrightnessRequest{
						Msg: GoveeBrightnessRequestMsg{
							Cmd: "brightness",
							Data: GoveeBrightnessRequestMsgData{
								Value: brightnessToSend,
							},
						},
					})
				}
				if message != nil {
					goveeMessages = append(goveeMessages, GoveeMessage{
						Device: goveeAction.Device,
						Data:   message,
					})
					if brightnessValueChanged == -1 {
						Status.SetBrightness(goveeAction.Device, brightnessValueChanged)
						Brightness.SetForDevice(goveeAction.Device, brightnessValueChanged)
					}
				}
			}
			for _, switchbotAction := range action.SwitchbotActions {
				message := NewSwitchbotMessageForDevice(switchbotAction.Device)
				switch switchbotAction.SyncValue {
				case LightSyncValueBrightness:
					brightnessToSend := brightness
					if len(switchbotAction.BrightnessRange) == 2 {
						brightnessToSend = getAdjustedBrightnessByRange(brightness, switchbotAction.BrightnessRange)
					}
					message = message.SetBrightness(brightnessToSend)
					Status.SetBrightness(switchbotAction.Device, brightnessToSend)
				}
				if !message.IsEmpty() {
					switchbotMessages = append(switchbotMessages, message)
				}
			}
			for _, wledAction := range action.WledActions {
				message := NewWledMessageForDevice(wledAction.Device)
				switch wledAction.SyncValue {
				case LightSyncValueBrightness:
					brightnessToSend := brightness
					if len(wledAction.BrightnessRange) == 2 {
						brightnessToSend = getAdjustedBrightnessByRange(brightness, wledAction.BrightnessRange)
					}
					brightnessToSend = mapBrightness(brightnessToSend, []int{0, 100}, []int{0, 255})
					message = message.SetBrightness(brightnessToSend)
					Status.SetBrightness(wledAction.Device, brightnessToSend)
				}
				if !message.IsEmpty() {
					wledMessages = append(wledMessages, message)
				}
			}
		}
	}
	return goveeMessages, switchbotMessages, wledMessages
}

func mapBrightness(brightness int, inRange []int, outRange []int) int {
	// 1. Check for valid range lengths
	if len(inRange) != 2 || len(outRange) != 2 {
		return brightness
	}

	inRangeDelta := inRange[1] - inRange[0]
	outRangeDelta := outRange[1] - outRange[0]

	// 2. Handle the division by zero case (when input range is a single point)
	if inRangeDelta == 0 {
		// Return the lower bound of the output range if the input is within the single-point range,
		// otherwise return the input, though a single-point range is usually illogical.
		return outRange[0]
	}

	// Convert input and deltas to float64 for precise calculation
	inVal := float64(brightness)
	inMin := float64(inRange[0])
	outMin := float64(outRange[0])

	// Calculate the normalized position (0.0 to 1.0)
	brightnessPercentageDelta := (inVal - inMin) / float64(inRangeDelta)

	// Perform the linear mapping
	mappedBrightnessFloat := outMin + (brightnessPercentageDelta * float64(outRangeDelta))

	// 3. Use math.Round() for accurate integer conversion, then clamp the value
	// Clamping ensures the result doesn't go outside the desired output range.
	mappedBrightness := int(math.Round(math.Min(math.Max(mappedBrightnessFloat, outMin), float64(outRange[1]))))

	return mappedBrightness
}

func getAdjustedBrightnessByRange(inBrightness int, brightnessRange []int) int {
	if len(brightnessRange) != 2 {
		return inBrightness
	}
	brightnessRangeDelta := brightnessRange[1] - brightnessRange[0]
	brightnessPercentageDelta := float64(brightnessRangeDelta) * (float64(inBrightness) / 100)
	adjustedBrightness := int(float64(brightnessRange[0]) + brightnessPercentageDelta)
	adjustedBrightness = int(math.Min(math.Max(float64(adjustedBrightness), 0), 100))
	return adjustedBrightness
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
					Status.SetOn(goveeAction.Device, true)
					Status.SetColor(goveeAction.Device, int(r), int(g), int(b))
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

func (c *Configuration) GetRequiredPresenceSensors() []string {
	var sensors []string
	for _, action := range c.Actions {
		if action.Trigger == ActionTriggerPresenceSensor {
			sensors = append(sensors, action.PresenceSensorName)
		}
	}
	return sensors
}

func (c *Configuration) IsPresenceSensorPresent(sensorName string) (bool, error) {
	value, err := c.presenceSensorActionsCache.Get(sensorName)
	if err != nil {
		return false, fmt.Errorf("error getting presence sensor '%s' presence from cache: %w", sensorName, err)
	}
	if value == nil {
		return false, nil
	}
	return true, nil
}

func (c *Configuration) GetMessagesToDispatchOnHuePresenceSensorChange(name string, presence bool) ([]GoveeMessage, []TwinklyMessage, []SwitchbotMessage, []WledMessage) {
	// var goveeMessages []GoveeMessage
	// var twinklyMessages []TwinklyMessage
	// var switchbotMessages []SwitchbotMessage
	// var wledMessages []WledMessage
	presenceSensorInterface, err := c.presenceSensorActionsCache.Get(name)
	if err != nil {
		log.Err(err).Msgf("Error getting presence sensor '%s' actions from cache: %s", name, err)
		return nil, nil, nil, nil
	}
	if presenceSensorInterface == nil {
		return nil, nil, nil, nil
	}
	// presenceSensorAction := presenceSensorInterface.(*ConfigurationAction)
	// TODO: Implement this
	return nil, nil, nil, nil
}

func (c *Configuration) GetDialNameByID(id string) (string, bool) {
	for name, dial := range c.Hue.Devices {
		if dial.ID == id {
			return name, true
		}
	}
	return "", false
}
