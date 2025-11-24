package main

import (
	"bytes"
	"fmt"
	"net/http"

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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	return nil
}
