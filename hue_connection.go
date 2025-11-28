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

	presenceSensorsStatuses sync.Map

	lightsStatuses map[string]LightStatus

	govee     *GoveeConnection
	twinkly   *TwinklyConnection
	switchbot *SwitchbotConnection
	wled      *WledConnection

	buttonPressedEventQueue  chan ButtonPressedEvent
	presenceSensorEventQueue chan PresenceSensorEvent
	lightChangeEventQueue    chan LightChangeEvent
}

type ButtonPressedEvent struct {
	DeviceName string
	Button     int
}

type PresenceSensorEvent struct {
	DeviceName string
	Presence   bool
}

type LightChangeEvent struct {
	DeviceName    string
	LastStatus    LightStatus
	CurrentStatus LightStatus
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

		buttonPressedEventQueue:  make(chan ButtonPressedEvent, 100),
		presenceSensorEventQueue: make(chan PresenceSensorEvent, 100),
		lightChangeEventQueue:    make(chan LightChangeEvent, 100),
	}
}

func (h *HueConnection) Start(
	ctx context.Context,
	configuration Configuration,
	govee *GoveeConnection,
	twinkly *TwinklyConnection,
	switchbot *SwitchbotConnection,
	wled *WledConnection,
) {
	// TODO: Implement action in case the bridge IP is empty
	// TODO: Implement action in case the bridge username is empty

	bridge := huego.New(h.bridgeIP, h.bridgeUsername)
	h.bridge = bridge

	h.govee = govee
	h.twinkly = twinkly
	h.switchbot = switchbot
	h.wled = wled

	go h.periodicallyPollSensors(ctx, configuration)

	go func() {
		for event := range h.buttonPressedEventQueue {

			goveeMessages, twinklyMessages, switchbotMessages, wledMessages := configuration.GetMessagesToDispatchOnHueTapDialButtonPressed(event.DeviceName, event.Button, h.wled)

			for _, message := range goveeMessages {
				if err := h.govee.SendMsg(message.Device, message.Data); err != nil {
					log.Err(err).Msg("error sending message")
				}
			}

			for _, message := range twinklyMessages {
				if err := h.twinkly.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending twinkly message")
				}
			}

			for _, message := range switchbotMessages {
				if err := h.switchbot.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending switchbot message")
				}
			}

			for _, message := range wledMessages {
				if err := h.wled.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending wled message")
				}
			}
		}
	}()

	go func() {
		for event := range h.presenceSensorEventQueue {
			goveeMessages, twinklyMessages, switchbotMessages, wledMessages := configuration.GetMessagesToDispatchOnHuePresenceSensorChange(event.DeviceName, event.Presence)

			for _, message := range goveeMessages {
				if err := h.govee.SendMsg(message.Device, message.Data); err != nil {
					log.Err(err).Msg("error sending message")
				}
			}

			for _, message := range twinklyMessages {
				if err := h.twinkly.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending twinkly message")
				}
			}

			for _, message := range switchbotMessages {
				if err := h.switchbot.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending switchbot message")
				}
			}

			for _, message := range wledMessages {
				if err := h.wled.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending wled message")
				}
			}
		}
	}()

	go func() {
		for event := range h.lightChangeEventQueue {

			lastStatus := event.LastStatus
			currentStatus := event.CurrentStatus

			var goveeMessages []GoveeMessage
			var twinklyMessages []TwinklyMessage
			var switchbotMessages []SwitchbotMessage
			var wledMessages []WledMessage

			if lastStatus.lastUpdate == nil {

				r, g, b := xyToRGB(currentStatus.x, currentStatus.y, float64(mapBrightness(currentStatus.brightness, []int{0, 100}, []int{0, 255})))

				log.Debug().Msgf("Light [%s] state changed to [on: %v, bri: %d, xy:<%f,%f>, rgb:<%d,%d,%d>]", event.DeviceName, currentStatus.on, currentStatus.brightness, currentStatus.x, currentStatus.y, r, g, b)

				goveeMessages, twinklyMessages, switchbotMessages, wledMessages = configuration.GetMessagesToDispatchOnHueLightOnOffChange(event.DeviceName, currentStatus.on)
				goveeMessages = append(goveeMessages, configuration.GetMessagesToDispatchOnHueLightColorChange(event.DeviceName, r, g, b)...)

			} else if lastStatus.on != currentStatus.on {

				log.Debug().Msgf("Light [%s] state changed to [on: %v]", event.DeviceName, currentStatus.on)
				goveeMessages, twinklyMessages, switchbotMessages, wledMessages = configuration.GetMessagesToDispatchOnHueLightOnOffChange(event.DeviceName, currentStatus.on)

			} else if lastStatus.brightness != currentStatus.brightness {

				log.Debug().Msgf("Light [%s] state changed to [bri: %d]", event.DeviceName, currentStatus.brightness)

				goveeMessages, switchbotMessages, wledMessages = configuration.GetMessagesToDispatchOnHueLightBrightnessChange(event.DeviceName, currentStatus.brightness)

			} else if lastStatus.x != currentStatus.x || lastStatus.y != currentStatus.y {
				r, g, b := xyToRGB(currentStatus.x, currentStatus.y, float64(mapBrightness(currentStatus.brightness, []int{0, 100}, []int{0, 255})))

				log.Debug().Msgf("Light [%s] state changed to [xy:<%f,%f>, rgb:<%d,%d,%d>]", event.DeviceName, currentStatus.x, currentStatus.y, r, g, b)

				goveeMessages = configuration.GetMessagesToDispatchOnHueLightColorChange(event.DeviceName, r, g, b)
			}

			for _, message := range goveeMessages {
				if err := h.govee.SendMsg(message.Device, message.Data); err != nil {
					log.Err(err).Msg("error sending govee message")
				}
			}

			for _, message := range twinklyMessages {
				if err := h.twinkly.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending twinkly message")
				}
			}

			for _, message := range switchbotMessages {
				if err := h.switchbot.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending switchbot message")
				}
			}

			for _, message := range wledMessages {
				if err := h.wled.SendMsg(message); err != nil {
					log.Err(err).Msg("error sending wled message")
				}
			}
		}
	}()

	h.pollState(ctx, configuration)
}

func (h *HueConnection) pollState(
	ctx context.Context,
	configuration Configuration,
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

						h.buttonPressedEventQueue <- ButtonPressedEvent{
							DeviceName: dial.Name,
							Button:     buttonPressed,
						}
					}
				}
				{
					if found, err := configuration.IsPresenceSensorPresent(name); err == nil && found {
						sensorStatusInterface, _ := h.presenceSensorsStatuses.LoadOrStore(name, PresenceSensorStatus{
							presence:    false,
							lastUpdated: nil,
						})
						sensorStatus := sensorStatusInterface.(PresenceSensorStatus)

						sensorState := sensorValue["state"].(map[string]interface{})

						lastUpdatedState := sensorState["lastupdated"].(string)
						var lastUpdated time.Time
						if lastUpdatedState == "none" {
							lastUpdated = time.Time{}
							continue
						} else {
							lastUpdated, err = time.Parse("2006-01-02T15:04:05", lastUpdatedState)
							if err != nil {
								log.Err(err).Msgf("error parsing time on presence sensor [%s]", name)
								continue
							}
						}

						presenceAvailable := sensorState["presence"] != nil
						if !presenceAvailable {
							continue
						}

						presence := sensorState["presence"].(bool)

						if sensorStatus.lastUpdated != nil && sensorStatus.lastUpdated.Equal(lastUpdated) && presence == sensorStatus.presence {
							continue
						}

						sensorStatus.lastUpdated = &lastUpdated
						sensorStatus.presence = presence
						h.presenceSensorsStatuses.Store(name, sensorStatus)

						log.Debug().Msgf("Presence sensor [%s] changed to [%v]", name, presence)

						h.presenceSensorEventQueue <- PresenceSensorEvent{
							DeviceName: name,
							Presence:   presence,
						}
					} else if err != nil {
						log.Err(err).Msgf("error checking presence sensor [%s] requirement", name)
					}
				}
			}

			lights := fullBridgeState["lights"].(map[string]interface{})

			for _, rawLightValue := range lights {
				light := rawLightValue.(map[string]interface{})

				rawDeviceName := light["name"]
				deviceName := rawDeviceName.(string)

				required := configuration.IsLightRequired(deviceName)

				if required {
					lightStatus := h.lightsStatuses[deviceName]

					lightState := light["state"].(map[string]interface{})

					on := lightState["on"].(bool)

					var rawBrightness float64 = 255
					var brightness int = 100
					brightnessAvailable := lightState["bri"] != nil

					if brightnessAvailable {
						rawBrightness = lightState["bri"].(float64)
						brightness = int(math.Max(math.Min((rawBrightness/255)*100, 100), 0))
						Brightness.SetForDevice(deviceName, brightness)
					}

					var x float64 = 0
					var y float64 = 0
					colorAvailable := lightState["xy"] != nil

					if colorAvailable {
						xy := lightState["xy"].([]interface{})
						x = xy[0].(float64)
						y = xy[1].(float64)
					}

					now := time.Now()
					currentStatus := LightStatus{
						lastUpdate: &now,
						on:         on,
						brightness: brightness,
						x:          x,
						y:          y,
					}

					h.lightsStatuses[deviceName] = currentStatus

					h.lightChangeEventQueue <- LightChangeEvent{
						DeviceName:    deviceName,
						LastStatus:    lightStatus,
						CurrentStatus: currentStatus,
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
	// presenceSensorsLocked := false

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
		case "ZLLPresence":
			// if !presenceSensorsLocked {
			// 	h.presenceSensorsMutex.Lock()
			// 	presenceSensorsLocked = true
			// }
			// h.presenceSensors[sensor.Name] = sensor
		}
	}

	if dialsLocked {
		h.dialsMutex.Unlock()
	}
	if dialRotariesLocked {
		h.dialRotariesMutex.Unlock()
	}
	// if presenceSensorsLocked {
	// 	h.presenceSensorsMutex.Unlock()
	// }
}
