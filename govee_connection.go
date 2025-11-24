package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type GoveeCommandSender interface {
	SendMsg(device string, data []byte) error
}

type GoveeConnection struct {
	configuration Configuration

	goveeDevices                map[string]*FoundGoveeDevice
	goveeDevicesOfInterestMutex struct{ sync.Mutex }

	goveeDevicesStatus      map[string]*GoveeDeviceStatus
	goveeDevicesStatusMutex struct{ sync.Mutex }
}

var _ GoveeCommandSender = (*GoveeConnection)(nil)

func NewGoveeConnection(
	configuration Configuration,
) *GoveeConnection {

	goveeDevices := configuration.GetRequiredGoveeDevices()
	goveeDevicesOfInterest := make(map[string]*FoundGoveeDevice, len(goveeDevices))
	for _, device := range goveeDevices {
		goveeDevicesOfInterest[device] = nil
	}

	return &GoveeConnection{
		goveeDevices:       goveeDevicesOfInterest,
		goveeDevicesStatus: make(map[string]*GoveeDeviceStatus),
		configuration:      configuration,
	}
}

func (c *GoveeConnection) SendMsg(device string, data []byte) error {
	c.goveeDevicesOfInterestMutex.Lock()
	defer c.goveeDevicesOfInterestMutex.Unlock()
	deviceRegistered, ok := c.goveeDevices[device]
	if !ok || deviceRegistered == nil {
		return fmt.Errorf("device not found")
	}
	return deviceRegistered.Send(data)
}

func (c *GoveeConnection) Start(ctx context.Context) error {
	resp := make(chan GoveeGenericResponse, 20)

	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return fmt.Errorf("error resolving server UDP address: %w", err)
	}

	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return fmt.Errorf("error starting UDP server: %w", err)
	}

	go func() {
		// Buffer to hold received data
		buffer := make([]byte, 1024)

		// Infinite loop to listen for responses
		for {
			select {
			case <-ctx.Done():
				serverConn.Close()
				return
			default:
				n, _, err := serverConn.ReadFromUDP(buffer)
				if err != nil {
					log.Err(err).Msgf("Error reading UDP response: %s", err)
					continue
				}

				// Parse the received JSON response
				var response GoveeGenericResponse
				err = json.Unmarshal(buffer[:n], &response)
				if err != nil {
					log.Err(err).Msgf("Error decoding JSON response: %s", err)
					continue
				}

				resp <- response
			}

		}
	}()

	c.listenToUDPMessages(ctx, resp)

	return nil
}

// TODO: DELETE (DEPRECATED)
func startUDPServer(ctx context.Context) (<-chan GoveeGenericResponse, func() error, error) {

	resp := make(chan GoveeGenericResponse, 20)

	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving server UDP address: %w", err)
	}

	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("error starting UDP server: %w", err)
	}

	go func() {
		// Buffer to hold received data
		buffer := make([]byte, 1024)

		// Infinite loop to listen for responses
		for {
			select {
			case <-ctx.Done():
				serverConn.Close()
				return
			default:
				n, _, err := serverConn.ReadFromUDP(buffer)
				if err != nil {
					log.Err(err).Msgf("Error reading UDP response: %s", err)
					continue
				}

				// Parse the received JSON response
				var response GoveeGenericResponse
				err = json.Unmarshal(buffer[:n], &response)
				if err != nil {
					log.Err(err).Msgf("Error decoding JSON response: %s", err)
					continue
				}

				resp <- response
			}

		}
	}()

	return resp, serverConn.Close, nil
}

func (c *GoveeConnection) listenToUDPMessages(ctx context.Context, receiveFromGovee <-chan GoveeGenericResponse) {
	for {
		select {
		case <-ctx.Done():
			return
		case response, ok := <-receiveFromGovee:
			if !ok {
				return
			}
			switch response.Msg.Cmd {
			case "scan":
				data := response.Msg.Data.(map[string]interface{})
				ip := data["ip"].(string)
				device := data["device"].(string)
				sku := data["sku"].(string)

				c.goveeDevicesOfInterestMutex.Lock()
				previousGoveeDeviceRegistered, ok := c.goveeDevices[device]
				// if ok && previousGoveeDeviceRegistered != nil {
				// 	if previousGoveeDeviceRegistered.SKU != sku ||
				// 		previousGoveeDeviceRegistered.IP != ip {
				// 		log.Info().Msgf(
				// 			"Device [%s - %s - %s] already registered: moving to [%s - %s - %s]",
				// 			previousGoveeDeviceRegistered.SKU, previousGoveeDeviceRegistered.IP, previousGoveeDeviceRegistered.Device,
				// 			sku, ip, device)

				// 		// Close and cleanup connection
				// 		previousGoveeDeviceRegistered.connMutex.Lock()
				// 		previousGoveeDeviceRegistered.conn.Close()
				// 		previousGoveeDeviceRegistered.channelOpen = false
				// 		if previousGoveeDeviceRegistered.sendChan != nil {
				// 			close(previousGoveeDeviceRegistered.sendChan)
				// 		}
				// 		previousGoveeDeviceRegistered.connMutex.Unlock()
				// 	}
				// } else {
				// 	log.Info().Msgf("Found Govee device [%s - %s - %s]", sku, ip, device)
				// }

				if !ok || previousGoveeDeviceRegistered == nil {
					log.Info().Msgf("Found Govee device [%s - %s - %s]", sku, ip, device)

					// Register the device
					deviceRegistered := &FoundGoveeDevice{
						IP:           ip,
						SKU:          sku,
						Device:       device,
						RegisteredAt: time.Now(),
					}
					c.goveeDevices[device] = deviceRegistered

					// Try to dial the device
					go tryDialGoveeDevice(ctx, deviceRegistered)
				}
				c.goveeDevicesOfInterestMutex.Unlock()
			case "devStatus":
				data := response.Msg.Data.(map[string]float64)
				brightness := data["brightness"]
				on := data["onOff"] == 1

				// Update the status
				c.goveeDevicesStatusMutex.Lock()
				status, ok := c.goveeDevicesStatus[response.Msg.Cmd]
				if !ok {
					status = &GoveeDeviceStatus{}
				}
				status.Brightness = brightness
				status.On = on
				c.goveeDevicesStatus[response.Msg.Cmd] = status
				c.goveeDevicesStatusMutex.Unlock()
			}
		}
	}
}

func tryDialGoveeDevice(ctx context.Context, device *FoundGoveeDevice) {
	device.connMutex.Lock()
	defer device.connMutex.Unlock()

	if device.channelOpen {
		return
	}

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", device.IP, sendPort))
	if err != nil {
		err = fmt.Errorf("error dialing: %w", err)
		log.Err(err).Msgf("Error connecting to Govee device [%s - %s - %s]", device.SKU, device.IP, device.Device)
		return
	}

	device.sendChan = make(chan []byte, 20)
	device.conn = conn
	device.channelOpen = true

	go startSendingMessages(ctx, device)
}

func startSendingMessages(ctx context.Context, device *FoundGoveeDevice) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-device.sendChan:
			if !ok {
				return
			}
			device.connMutex.Lock()
			if !device.channelOpen {
				log.Error().Msgf("Channel closed for device [%s - %s - %s]; cannot forward the following message: %s",
					device.SKU, device.IP, device.Device,
					string(data))
				device.connMutex.Unlock()
				return
			}
			log.Debug().Msgf("Forwarding message to Govee device [%s - %s - %s]: %s", device.SKU, device.IP, device.Device, string(data))
			_, err := device.conn.Write(data)
			if err != nil {
				err = fmt.Errorf("error writing to connection: %w", err)
				log.Err(err).Msgf("Error forwarding message to Govee device [%s - %s - %s]", device.SKU, device.IP, device.Device)
				// TODO: (Improvement) close the connection and recycle the device after some failed attempts
				// device.conn.Close()
				// device.channelOpen = false
				// close(device.sendChan)
			}
			device.connMutex.Unlock()
		}
	}
}
