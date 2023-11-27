package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"github.com/amimof/huego"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// https://app-h5.govee.com/user-manual/wlan-guide

const (
	multicastAddress = "239.255.255.250"
	broadcastPort    = 4001
	listenPort       = 4002
	sendPort         = 4003
	appName          = "hue-govee synchronizer"
	pollingDuration  = time.Duration(200 * time.Millisecond)
)

var goveeBrightness float64 = 100

func main() {

	// Init logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Get CLI param
	chosenSKU := flag.String("sku", "", "sku of the Govee light")
	bridgeIP := flag.String("bridge", "", "ip of the Philips Hue bridge")
	bridgeUsername := flag.String("username", "", "username of the Philips Hue bridge")
	flag.Parse()

	ctx := context.Background()

	closeUDPServer, receiveFromGovee, err := startUDPServer(ctx)
	if err != nil {
		panic(err)
	}
	defer closeUDPServer()

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", multicastAddress, broadcastPort))
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		fmt.Println("Error joining multicast group:", err)
		return
	}
	defer conn.Close()

	scanRequest := GoveeScanRequest{
		Msg: GoveeScanRequestMsg{
			Cmd: "scan",
			Data: GoveeScanRequestMsgData{
				AccountTopic: "reserve",
			},
		},
	}

	requestJSON, err := json.Marshal(scanRequest)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	_, err = conn.WriteTo(requestJSON, addr)
	if err != nil {
		fmt.Println("Error sending 'request scan' message:", err)
		return
	}

	fmt.Println("Request sent successfully.")

	if *chosenSKU == "" {
		log.Warn().Msgf("Light SKU not specified: printing out all retrieved and closing in 20 seconds")

		go func() {
			for msg := range receiveFromGovee {
				fmt.Println(msg)
				// log.Info().Msgf("IP: %s <-> MAC: %s <-> SKU: %s", msg.Msg.Data.IP, msg.Msg.Data.Device, msg.Msg.Data.SKU)
			}
		}()
		time.Sleep(20 * time.Second)
		return
	}

	sendToGovee := make(chan []byte, 10)

	go func() {

		reader := bufio.NewReader(os.Stdin)

		for {
			str, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}

			str = strings.TrimSpace(str)

			switch str {
			case "on":
				sendToGovee <- mustMarshal(GoveeTurn{
					Msg: GoveeTurnMsg{
						Cmd: "turn",
						Data: GoveeTurnMsgData{
							Value: 1,
						},
					},
				})
			case "off":
				sendToGovee <- mustMarshal(GoveeTurn{
					Msg: GoveeTurnMsg{
						Cmd: "turn",
						Data: GoveeTurnMsgData{
							Value: 0,
						},
					},
				})
			}
		}
	}()

	sendToGovee <- mustMarshal(GoveeStatusRequest{
		Msg: GoveeStatusRequestMsg{
			Cmd: "devStatus",
		},
	})

	go listenFromHueDevice(ctx, *bridgeIP, *bridgeUsername, sendToGovee)
	connectToGoveeDeviceAndForward(ctx, *chosenSKU, receiveFromGovee, sendToGovee)
}

func startUDPServer(ctx context.Context) (func(), <-chan GoveeGenericResponse, error) {

	resp := make(chan GoveeGenericResponse, 20)

	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving server UDP address: %w", err)
	}
	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("error starting UDP server: %w", err)
	}

	closeConnection := func() {
		serverConn.Close()
	}

	go func() {
		// Buffer to hold received data
		buffer := make([]byte, 1024)

		// Infinite loop to listen for responses
		for {
			select {
			case <-ctx.Done():
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

	return closeConnection, resp, nil
}

func listenFromHueDevice(ctx context.Context, bridgeIP string, bridgeUsername string, sendToGovee chan []byte) {

	var bridge *huego.Bridge

	if bridgeIP == "" {
		log.Info().Msgf("Bridge IP not specified: discovering..")
		var err error
		bridge, err = huego.Discover()
		if err != nil {
			panic(err)
		}
		bridgeIP = bridge.Host
		log.Info().Msgf("Bridge IP: %s", bridgeIP)
	}

	if bridgeUsername == "" {
		log.Info().Msgf("Bridge username not specified: press the button on the bridge to register a new one")

		attempts := 0

		for attempts < 12 {
			var err error
			bridgeUsername, err = bridge.CreateUser(appName)
			if err != nil {
				if !errorIsLinkButtonNotPressed(err) {
					panic(err)
				}
				time.Sleep(5 * time.Second)
				attempts++
			}
			if bridgeUsername != "" {
				log.Info().Msgf("Bridge username: %s", bridgeUsername)
				break
			}
		}

		if bridgeUsername == "" {
			log.Error().Msgf("Connection with the bridge cannot be made")
		}
	}

	bridge = huego.New(bridgeIP, bridgeUsername)

	sensors, err := bridge.GetSensors()
	if err != nil {
		panic(err)
	}

	var tapDial huego.Sensor
	var tapDialRotary huego.Sensor

	for _, sensor := range sensors {
		// fmt.Printf("%#v\n", sensor)
		switch sensor.Type {
		case "ZLLSwitch":
			tapDial = sensor
		case "ZLLRelativeRotary":
			tapDialRotary = sensor
		}
	}

	fmt.Println(tapDial, tapDialRotary)

	// config, err := bridge.GetConfig()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("config: %#v\n\n", config)

	var dialLastUpdate *time.Time
	var dialButtonEvent float64

	var dialRotaryLastUpdate *time.Time
	var dialRotaryExpectedRotation float64

	for {
		state, err := bridge.GetFullStateContext(ctx)
		if err != nil {
			log.Err(err).Msgf("Cannot get full state context: %s", err)
			continue
		}

		for _, v := range state["sensors"].(map[string]interface{}) {
			sensorValue := v.(map[string]interface{})
			if uniqueid, found := sensorValue["uniqueid"]; found {

				switch uniqueid {
				case tapDial.UniqueID:

					state := sensorValue["state"].(map[string]interface{})

					t, err := time.Parse("2006-01-02T15:04:05", state["lastupdated"].(string))
					if err != nil {
						panic(err)
					}

					button := state["buttonevent"].(float64)
					if dialLastUpdate == nil {
						dialLastUpdate = &t
						dialButtonEvent = button
					} else {
						if !dialLastUpdate.Equal(t) || button != dialButtonEvent {
							dialLastUpdate = &t
							dialButtonEvent = button
							// Pressed
							switch dialButtonEvent {
							case 1002:
								log.Info().Msgf("Turning on Govee light")
								sendToGovee <- mustMarshal(GoveeTurn{
									Msg: GoveeTurnMsg{
										Cmd: "turn",
										Data: GoveeTurnMsgData{
											Value: 1,
										},
									},
								})
							case 4002:
								log.Info().Msgf("Turning off Govee light")
								sendToGovee <- mustMarshal(GoveeTurn{
									Msg: GoveeTurnMsg{
										Cmd: "turn",
										Data: GoveeTurnMsgData{
											Value: 0,
										},
									},
								})
							}
						}
					}
				case tapDialRotary.UniqueID:
					state := sensorValue["state"].(map[string]interface{})

					t, err := time.Parse("2006-01-02T15:04:05", state["lastupdated"].(string))
					if err != nil {
						panic(err)
					}

					expectedRotation := state["expectedrotation"].(float64)
					if dialRotaryLastUpdate == nil {
						dialRotaryLastUpdate = &t
						dialRotaryExpectedRotation = expectedRotation
					} else {
						if !dialRotaryLastUpdate.Equal(t) || expectedRotation != dialRotaryExpectedRotation {
							dialRotaryLastUpdate = &t
							dialRotaryExpectedRotation = expectedRotation
							// Rotated
							previousBrightness := goveeBrightness
							goveeBrightness = math.Min(math.Max(goveeBrightness+(expectedRotation/8), 0), 100)

							if previousBrightness == 0 && goveeBrightness > 0 {
								// Turning on the light
								log.Info().Msgf("Turning on Govee light")
								sendToGovee <- mustMarshal(GoveeTurn{
									Msg: GoveeTurnMsg{
										Cmd: "turn",
										Data: GoveeTurnMsgData{
											Value: 1,
										},
									},
								})
							} else if previousBrightness > 0 && goveeBrightness == 0 {
								// Turning off the light
								log.Info().Msgf("Turning off Govee light")
								sendToGovee <- mustMarshal(GoveeTurn{
									Msg: GoveeTurnMsg{
										Cmd: "turn",
										Data: GoveeTurnMsgData{
											Value: 0,
										},
									},
								})
							}

							if previousBrightness != goveeBrightness && goveeBrightness != 0 {
								log.Info().Msgf("Setting brightness to %.0f", goveeBrightness)
								sendToGovee <- mustMarshal(GoveeBrightnessRequest{
									Msg: GoveeBrightnessRequestMsg{
										Cmd: "brightness",
										Data: GoveeBrightnessRequestMsgData{
											Value: int(goveeBrightness),
										},
									},
								})
							}
						}
					}
					// fmt.Printf("%#v\n", state)

					// for k, v := range state {
					// 	fmt.Printf("key: %#v\n", k)
					// 	fmt.Printf("value: %#v\n\n", v)
					// }
				}

				// for k, v := range state {

				// 	fmt.Printf("key: %#v\n", k)
				// 	fmt.Printf("value: %#v\n\n", v)
				// }
			}
		}

		time.Sleep(pollingDuration)
	}

	fmt.Println(dialLastUpdate, dialButtonEvent)

	// fmt.Println("sensors", sensors)
}

func connectToGoveeDeviceAndForward(ctx context.Context, sku string, receiveFromGovee <-chan GoveeGenericResponse, sendToGovee chan []byte) {
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ip := ""

L:
	for {
		select {
		case msg, ok := <-receiveFromGovee:
			if !ok {
				return
			}
			fmt.Println("here:", msg)
			switch msg.Msg.Cmd {
			case "scan":
				data := msg.Msg.Data.(map[string]interface{})
				foundSKU := data["sku"].(string)
				foundIP := data["ip"].(string)
				if sku == foundSKU {
					log.Info().Msgf("Received device %s response.", sku)
					ip = foundIP
					break L
				}
			default:
				log.Warn().Msgf("Received a message of type %s but the program is not in a valid state to handle this kind of response", msg.Msg.Cmd)
			}
		case <-timeout.Done():
			log.Warn().Msgf("Response not received in 10 seconds: closing.")
			return
		}
	}

	fmt.Printf("Command to send to %s:\n", ip)

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", ip, sendPort))
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case msg, ok := <-receiveFromGovee:
				if !ok {
					return
				}
				switch msg.Msg.Cmd {
				case "devStatus":
					data := msg.Msg.Data.(map[string]interface{})
					brightness := data["brightness"].(float64)
					goveeBrightness = brightness
					log.Info().Msgf("Brightness of the device: %f", brightness)
				default:
					log.Warn().Msgf("Response type %#v not supported", msg.Msg.Cmd)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for msg := range sendToGovee {
		log.Debug().Msgf("Sending datagram to %s: %s", ip, string(msg))
		_, err := conn.Write(msg)
		if err != nil {
			log.Err(err).Msgf("Cannot write datagram: %s", err)
		}
	}
}

func mustMarshal(obj any) []byte {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}

func errorIsLinkButtonNotPressed(err error) bool {
	return strings.Contains(err.Error(), "link button not pressed")
}
