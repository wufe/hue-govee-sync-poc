package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/amimof/huego"
	"github.com/rs/zerolog/log"
)

type HueConnection struct {
	bridgeIP       string
	bridgeUsername string

	bridge *huego.Bridge

	dials         map[string]huego.Sensor
	dialsMutex    struct{ sync.RWMutex }
	dialsStatuses map[string]DialStatus

	dialRotaries         map[string]huego.Sensor
	dialRotariesMutex    struct{ sync.RWMutex }
	dialRotariesStatuses map[string]DialRotaryStatus

	lightsStatuses map[string]LightStatus
}

func NewHueConnection(
	bridgeIP string,
	bridgeUsername string,
) HueConnection {
	return HueConnection{
		dials:         make(map[string]huego.Sensor),
		dialsStatuses: make(map[string]DialStatus),

		dialRotaries:         make(map[string]huego.Sensor),
		dialRotariesStatuses: make(map[string]DialRotaryStatus),

		lightsStatuses: make(map[string]LightStatus),

		bridgeIP:       bridgeIP,
		bridgeUsername: bridgeUsername,
	}
}

func (h *HueConnection) Start(
	ctx context.Context,
	configuration Configuration,
	goveeCommandSender GoveeCommandSender,
	twinklyCommandSender TwinklyCommandSender,
	switchbotCommandSender SwitchbotCommandSender,
	wledCommandSender WledCommandSender,
) {
	// TODO: Implement action in case the bridge IP is empty
	// TODO: Implement action in case the bridge username is empty

	bridge := huego.New(h.bridgeIP, h.bridgeUsername)
	h.bridge = bridge

	go h.periodicallyPollSensors(ctx, configuration)

	h.pollState(ctx, configuration, goveeCommandSender, twinklyCommandSender, switchbotCommandSender, wledCommandSender)
}

func (h *HueConnection) pollState(
	ctx context.Context,
	configuration Configuration,
	goveeCommandSender GoveeCommandSender,
	twinklyCommandSender TwinklyCommandSender,
	switchbotCommandSender SwitchbotCommandSender,
	wledCommandSender WledCommandSender,
) {
	for {
		time.Sleep(200 * time.Millisecond)

		select {
		case <-ctx.Done():
			return
		default:
			fullBridgeState, err := h.bridge.GetFullStateContext(ctx)
			if err != nil {
				err = fmt.Errorf("error getting full state context: %w", err)
				log.Err(err).Msg(err.Error())
				continue
			}
			sensors := fullBridgeState["sensors"].(map[string]interface{})
			for _, rawSensorValue := range sensors {
				sensorValue := rawSensorValue.(map[string]interface{})

				rawName, found := sensorValue["name"]
				if !found {
					continue
				}

				name := rawName.(string)
				{
					h.dialsMutex.RLock()
					dial, found := h.dials[name]
					h.dialsMutex.RUnlock()
					if found {
						dialStatus := h.dialsStatuses[name]

						dialState := sensorValue["state"].(map[string]interface{})

						lastUpdatedState := dialState["lastupdated"].(string)
						var lastUpdated time.Time
						if lastUpdatedState == "none" {
							lastUpdated = time.Time{}
							continue
						} else {
							lastUpdated, err = time.Parse("2006-01-02T15:04:05", lastUpdatedState)
							if err != nil {
								log.Err(err).Msgf("error parsing time on dial [%s]", dial.Name)
								continue
							}
						}

						button := dialState["buttonevent"].(float64)
						if dialStatus.lastUpdate == nil {
							dialStatus.lastUpdate = &lastUpdated
							dialStatus.buttonEvent = button
							h.dialsStatuses[name] = dialStatus
							continue
						}

						if dialStatus.lastUpdate.Equal(lastUpdated) && button == dialStatus.buttonEvent {
							continue
						}

						dialStatus.lastUpdate = &lastUpdated
						dialStatus.buttonEvent = button
						h.dialsStatuses[name] = dialStatus

						log.Debug().Msgf("Button [%d] pressed on dial [%s]", int(dialStatus.buttonEvent), dial.Name)

						buttonPressed := int(dialStatus.buttonEvent)

						goveeMessages, twinklyMessages := configuration.GetMessagesToDispatchOnHueTapDialButtonPressed(dial.Name, buttonPressed)

						for _, message := range goveeMessages {
							if err := goveeCommandSender.SendMsg(message.Device, message.Data); err != nil {
								log.Err(err).Msg("error sending message")
							}
						}

						for _, message := range twinklyMessages {
							if err := twinklyCommandSender.SendMsg(message); err != nil {
								log.Err(err).Msg("error sending twinkly message")
							}
						}
					}
				}
			}

			lights := fullBridgeState["lights"].(map[string]interface{})

			for _, rawLightValue := range lights {
				light := rawLightValue.(map[string]interface{})

				rawName := light["name"]
				name := rawName.(string)

				required := configuration.IsLightRequired(name)

				if required {
					lightStatus := h.lightsStatuses[name]

					lightState := light["state"].(map[string]interface{})

					on := lightState["on"].(bool)
					rawBrightness := lightState["bri"].(float64)
					brightness := int(math.Max(math.Min((rawBrightness/255)*100, 100), 0))
					xy := lightState["xy"].([]interface{})
					x := xy[0].(float64)
					y := xy[1].(float64)

					var goveeMessages []GoveeMessage
					var twinklyMessages []TwinklyMessage
					var switchbotMessages []SwitchbotMessage
					var wledMessages []WledMessage

					if lightStatus.lastUpdate == nil {
						r, g, b := xyToRGB(x, y, rawBrightness)

						log.Debug().Msgf("Light [%s] state changed to [on: %v, bri: %d, xy:<%f,%f>, rgb:<%d,%d,%d>]", name, on, brightness, x, y, r, g, b)
						lightStatus.on = on
						lightStatus.brightness = brightness
						lightStatus.x = x
						lightStatus.y = y
						now := time.Now()
						lightStatus.lastUpdate = &now

						goveeMessages, twinklyMessages, switchbotMessages, wledMessages = configuration.GetMessagesToDispatchOnHueLightOnOffChange(name, on)
						goveeMessages = append(goveeMessages, configuration.GetMessagesToDispatchOnHueLightColorChange(name, r, g, b)...)
					} else if lightStatus.on != on {
						log.Debug().Msgf("Light [%s] state changed to [on: %v]", name, on)
						lightStatus.on = on
						now := time.Now()
						lightStatus.lastUpdate = &now

						goveeMessages, twinklyMessages, switchbotMessages, wledMessages = configuration.GetMessagesToDispatchOnHueLightOnOffChange(name, on)
					} else if lightStatus.brightness != brightness {
						log.Debug().Msgf("Light [%s] state changed to [bri: %d]", name, brightness)
						lightStatus.brightness = brightness
						now := time.Now()
						lightStatus.lastUpdate = &now

						goveeMessages, switchbotMessages, wledMessages = configuration.GetMessagesToDispatchOnHueLightBrightnessChange(name, brightness)
					} else if lightStatus.x != x || lightStatus.y != y {
						r, g, b := xyToRGB(x, y, rawBrightness)

						log.Debug().Msgf("Light [%s] state changed to [xy:<%f,%f>, rgb:<%d,%d,%d>]", name, x, y, r, g, b)
						lightStatus.x = x
						lightStatus.y = y
						now := time.Now()
						lightStatus.lastUpdate = &now

						goveeMessages = configuration.GetMessagesToDispatchOnHueLightColorChange(name, r, g, b)
					}

					h.lightsStatuses[name] = lightStatus

					for _, message := range goveeMessages {
						if err := goveeCommandSender.SendMsg(message.Device, message.Data); err != nil {
							log.Err(err).Msg("error sending govee message")
						}
					}

					for _, message := range twinklyMessages {
						if err := twinklyCommandSender.SendMsg(message); err != nil {
							log.Err(err).Msg("error sending twinkly message")
						}
					}

					for _, message := range switchbotMessages {
						if err := switchbotCommandSender.SendMsg(message); err != nil {
							log.Err(err).Msg("error sending switchbot message")
						}
					}

					for _, message := range wledMessages {
						if err := wledCommandSender.SendMsg(message); err != nil {
							log.Err(err).Msg("error sending wled message")
						}
					}
				}
			}
		}
	}
}

func (h *HueConnection) periodicallyPollSensors(ctx context.Context, configuration Configuration) {
	h.retrieveSensors(true, configuration)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			h.retrieveSensors(false, configuration)
		}
	}
}

func (h *HueConnection) retrieveSensors(firstLook bool, configuration Configuration) {
	sensors, err := h.bridge.GetSensors()
	if err != nil {
		log.Error().Err(err).Msg("error retrieving sensors")
		return
	}

	requiredDialsList := configuration.GetRequiredHueDials()

	requiredSensors := make(map[string]struct{}, len(requiredDialsList))
	for _, dial := range requiredDialsList {
		requiredSensors[dial] = struct{}{}
	}

	dialsLocked := false
	dialRotariesLocked := false

	for _, sensor := range sensors {
		if firstLook {
			log.Debug().Msgf("Found Hue sensor [%s]", sensor.Name)
		}
		if _, isRequired := requiredSensors[sensor.Name]; !isRequired {
			continue
		}
		switch sensor.Type {
		case "ZLLSwitch":

			if !dialsLocked {
				h.dialsMutex.Lock()
				dialsLocked = true
			}
			h.dials[sensor.Name] = sensor
		case "ZLLRelativeRotary":
			if !dialRotariesLocked {
				h.dialRotariesMutex.Lock()
				dialRotariesLocked = true
			}
			h.dialRotaries[sensor.Name] = sensor
		}
	}

	if dialsLocked {
		h.dialsMutex.Unlock()
	}
	if dialRotariesLocked {
		h.dialRotariesMutex.Unlock()
	}
}
