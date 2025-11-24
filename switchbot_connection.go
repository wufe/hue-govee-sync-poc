package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

type SwitchbotConnection struct {
	devices map[string]SwitchbotDeviceConfiguration
}

type SwitchbotCommandSender interface {
	SendMsg(message SwitchbotMessage) error
}

func NewSwitchbotConnection(configuration Configuration) *SwitchbotConnection {
	devices := configuration.Switchbot
	return &SwitchbotConnection{
		devices: devices,
	}
}

func (c *SwitchbotConnection) SendMsg(msg SwitchbotMessage) error {
	device, ok := c.devices[msg.Device]
	if !ok {
		return fmt.Errorf("device not found: %s", msg.Device)
	}

	body := fmt.Sprintf(`{"command":"%s","parameter":"%s","commandType":"%s"}`, msg.Command, msg.Parameter, msg.CommandType)

	log.Debug().
		Str("device", msg.Device).
		Str("body", body).
		Msgf("Sending message to switchbot device")

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.switch-bot.com/v1.0/devices/%s/commands", device.DeviceID), strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", device.Authorization.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	return nil
}
