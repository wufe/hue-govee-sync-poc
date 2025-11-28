package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type WledConnection struct {
	devices map[string]WledDeviceConfiguration // Key is friendly device name
}

type WledCommandSender interface {
	SendMsg(message WledMessage) error
}

func NewWledConnection(configuration Configuration) *WledConnection {
	devices := configuration.Wled
	return &WledConnection{
		devices: devices,
	}
}

func (c *WledConnection) SendMsg(msg WledMessage) error {
	device, ok := c.devices[msg.Device]
	if !ok {
		return fmt.Errorf("device not found: %s", msg.Device)
	}

	log.Debug().
		Str("device", msg.Device).
		Str("ip", device.IP).
		Msgf("Sending message to WLED device")

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/json/state", device.IP), bytes.NewReader(msg.Body))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

func (c *WledConnection) GetDeviceBrightness(deviceName string) (int, error) {
	device, ok := c.devices[deviceName]
	if !ok {
		return 0, fmt.Errorf("device not found: %s", deviceName)
	}

	log.Debug().
		Str("device", deviceName).
		Str("ip", device.IP).
		Msgf("Getting brightness from WLED device")

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/json/state", device.IP), nil)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	var state WledStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return 0, fmt.Errorf("error decoding response: %v", err)
	}

	return mapBrightness(state.Bri, []int{0, 255}, []int{0, 100}), nil
}

type WledStateResponse struct {
	On  bool `json:"on"`
	Bri int  `json:"bri"`
}
