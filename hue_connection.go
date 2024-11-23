package main

import (
	"context"
	"fmt"
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

		bridgeIP:       bridgeIP,
		bridgeUsername: bridgeUsername,
	}
}

func (h *HueConnection) Start(ctx context.Context, configuration Configuration, commandSender GoveeCommandSender) {
	// TODO: Implement action in case the bridge IP is empty
	// TODO: Implement action in case the bridge username is empty

	bridge := huego.New(h.bridgeIP, h.bridgeUsername)
	h.bridge = bridge

	go h.periodicallyPollSensors(ctx, configuration)

	h.pollState(ctx, configuration, commandSender)
}

func (h *HueConnection) pollState(ctx context.Context, configuration Configuration, commandSender GoveeCommandSender) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			state, err := h.bridge.GetFullStateContext(ctx)
			if err != nil {
				err = fmt.Errorf("error getting full state context: %w", err)
				log.Err(err).Msg(err.Error())
				continue
			}
			sensors := state["sensors"].(map[string]interface{})
			for _, rawSensorValue := range sensors {
				sensorValue := rawSensorValue.(map[string]interface{})
				uniqueID := sensorValue["uniqueid"].(string)

				{
					h.dialsMutex.RLock()
					dial, found := h.dials[uniqueID]
					h.dialsMutex.RUnlock()
					if found {
						dialStatus := h.dialsStatuses[uniqueID]

						dialState := sensorValue["state"].(map[string]interface{})

						lastUpdated, err := time.Parse(time.RFC3339, dialState["lastupdated"].(string))
						if err != nil {
							log.Err(err).Msg("error parsing time")
							continue
						}

						button := state["buttonevent"].(float64)
						if dialStatus.lastUpdate == nil {
							dialStatus.lastUpdate = &lastUpdated
							dialStatus.buttonEvent = button
							h.dialsStatuses[uniqueID] = dialStatus
							continue
						}

						if dialStatus.lastUpdate.Equal(lastUpdated) && button != dialStatus.buttonEvent {
							continue
						}

						dialStatus.lastUpdate = &lastUpdated
						dialStatus.buttonEvent = button
						h.dialsStatuses[uniqueID] = dialStatus

						log.Debug().Msgf("Button %d pressed on dial [%s]", int(dialStatus.buttonEvent), dial.Name)

						messages := configuration.GetMessagesToDispatchOnHueTapDialButtonPressed(dial.Name, int(dialStatus.buttonEvent))
						for _, message := range messages {
							if err := commandSender.SendMsg(message.Device, message.Data); err != nil {
								log.Err(err).Msg("error sending message")
							}
						}

					}
				}

			}
		}
	}
}

func (h *HueConnection) periodicallyPollSensors(ctx context.Context, configuration Configuration) {
	h.retrieveSensors(configuration)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			h.retrieveSensors(configuration)
		}
	}
}

func (h *HueConnection) retrieveSensors(configuration Configuration) {
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
