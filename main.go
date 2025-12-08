package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amimof/huego"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sanity-io/litter"
)

// https://app-h5.govee.com/user-manual/wlan-guide

const (
	listenPort      = 4002
	sendPort        = 4003
	appName         = "hue-govee synchronizer"
	pollingDuration = time.Duration(200 * time.Millisecond)
)

var goveeBrightness float64 = 100
var goveeOn = false
var goveeColorR, goveeColorG, goveeColorB uint8
var goveeColorK uint16
var listenToEvents = false
var dialToListenTo string

func main() {

	// Init logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Get CLI param
	listen := flag.Bool("listen", false, "listen to events from the Hue bridge")
	flag.Parse()

	if *listen {
		listenToEvents = true
	}

	ctx := context.Background()

	var wg sync.WaitGroup

	configuration := NewConfiguration()

	for _, device := range configuration.GetAllGoveeDeviceAliases() {
		Status.Register(device, "Govee")
	}

	for _, device := range configuration.GetAllSwitchbotDeviceAliases() {
		Status.Register(device, "Switchbot")
	}

	for _, device := range configuration.GetAllWledDeviceAliases() {
		Status.Register(device, "WLED")
	}

	for _, device := range configuration.GetAllTwinklyDeviceAliases() {
		Status.Register(device, "Twinkly")
	}

	goveeConnection := NewGoveeConnection(configuration)

	switchbotConnection := NewSwitchbotConnection(configuration)

	wledConnection := NewWledConnection(configuration)

	var twinklyConnection *TwinklyConnection

	if len(configuration.Twinkly) > 0 {
		// Considering only the first device
		// TODO: support multiple devices
		twinklyIP := ""
		for _, twinklyDeviceConfiguration := range configuration.Twinkly {
			twinklyIP = twinklyDeviceConfiguration.IP
			break
		}

		twinklyConnection = NewTwinklyConnection(twinklyIP)
		if err := twinklyConnection.Login(ctx, twinklyIP); err != nil {
			panic(fmt.Errorf("error logging in to Twinkly device: %v", err))
		}
		log.Info().Msgf("Logged in to Twinkly device at %s", twinklyIP)
	} else {
		twinklyConnection = NewNoopTwinklyConnection()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		goveeConnection.Start(ctx)
	}()

	multicastConn, err := openMulticastConnection()
	if err != nil {
		panic(err)
	}
	defer multicastConn.Close()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := sendScanRequest(multicastConn); err != nil {
					log.Err(err).Msgf("Error sending scan request: %s", err)
				}
				time.Sleep(30 * time.Second) // Wait 10 seconds before sending the next scan request
			}
		}
	}()

	hueConnection := NewHueConnection(configuration.Hue.Bridge.IP, configuration.Hue.Bridge.Username)

	wg.Add(1)
	go func() {
		defer wg.Done()
		hueConnection.Start(ctx, configuration, goveeConnection, twinklyConnection, switchbotConnection, wledConnection)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := StartHTTPServer(); err != nil {
			log.Err(err).Msgf("HTTP server error: %s", err)
		}
	}()

	wg.Wait()
}

func listenFromHueDevice(ctx context.Context, bridgeIP string, bridgeUsername string, sendToGovee chan []byte, sender GoveeCommandSender) {

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

	type dialEventStatus struct {
		lastUpdate  *time.Time
		buttonEvent float64
	}

	type dialRotatoryEventStatus struct {
		lastUpdate       *time.Time
		expectedRotation float64
	}

	tapDialsToListenTo := make(map[string]huego.Sensor)
	tapDialsStatuses := make(map[string]dialEventStatus)

	tapDialRotatoriesToListenTo := make(map[string]huego.Sensor)
	tapDialRotatoriesStatuses := make(map[string]dialRotatoryEventStatus)

	for _, sensor := range sensors {
		// litter.Dump(sensor)
		switch sensor.Type {
		case "ZLLSwitch":
			tapDialsToListenTo[sensor.UniqueID] = sensor
			log.Debug().Msgf("Added a tap dial for listening named %s with uniqueID %s", sensor.Name, sensor.UniqueID)
		case "ZLLRelativeRotary":
			tapDialRotatoriesToListenTo[strings.ToLower(strings.TrimSpace(sensor.UniqueID))] = sensor
			log.Debug().Msgf("Added a tap dial rotary for listening named %s with uniqueID %s", sensor.Name, sensor.UniqueID)
		}
	}

	// config, err := bridge.GetConfig()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("config: %#v\n\n", config)

	for {
		state, err := bridge.GetFullStateContext(ctx)
		if err != nil {
			log.Err(err).Msgf("Cannot get full state context: %s", err)
			continue
		}

		// litter.Dump(state)

		// b, _ := json.Marshal(state)

		// fmt.Printf("\n\n\n%#v\n\n", string(b))
		// os.Exit(0)

		if listenToEvents {
			for _, v := range state["sensors"].(map[string]interface{}) {
				sensorValue := v.(map[string]interface{})
				if uniqueidInterface, found := sensorValue["uniqueid"]; found {

					uniqueID := strings.ToLower(strings.TrimSpace(uniqueidInterface.(string)))

					// fmt.Println(uniqueID)
					if tapDial, found := tapDialsToListenTo[uniqueID]; found {
						if tapDial.UniqueID == uniqueidInterface {
							state := sensorValue["state"].(map[string]interface{})

							t, err := time.Parse("2006-01-02T15:04:05", state["lastupdated"].(string))
							if err != nil {
								panic(err)
							}

							status := tapDialsStatuses[uniqueID]

							button := state["buttonevent"].(float64)
							if status.lastUpdate == nil {
								status.lastUpdate = &t
								status.buttonEvent = button
							} else {
								if !status.lastUpdate.Equal(t) || button != status.buttonEvent {
									status.lastUpdate = &t
									status.buttonEvent = button
									log.Debug().Msgf("Button %d pressed on dial [%s]", int(status.buttonEvent), tapDial.Name)

									if dialToListenTo == "" || tapDial.Name == dialToListenTo {
										// Pressed
										switch status.buttonEvent {
										case 1002, 2002:
											log.Info().Msgf("Turning on Govee light")
											msg := mustMarshal(GoveeTurn{
												Msg: GoveeTurnMsg{
													Cmd: "turn",
													Data: GoveeTurnMsgData{
														Value: 1,
													},
												},
											})
											if sender != nil {
												sender.SendMsg("33:1E:D6:38:32:31:2A:3A", msg)
											}
											sendToGovee <- msg
										case 3002, 4002:
											log.Info().Msgf("Turning off Govee light")
											msg := mustMarshal(GoveeTurn{
												Msg: GoveeTurnMsg{
													Cmd: "turn",
													Data: GoveeTurnMsgData{
														Value: 0,
													},
												},
											})
											if sender != nil {
												sender.SendMsg("33:1E:D6:38:32:31:2A:3A", msg)
											}
											sendToGovee <- msg
										}
									}
								}
							}

							tapDialsStatuses[uniqueID] = status
						}
					}

					if tapDialRotary, found := tapDialRotatoriesToListenTo[uniqueID]; found {
						if tapDialRotary.UniqueID == uniqueidInterface {
							state := sensorValue["state"].(map[string]interface{})

							t, err := time.Parse("2006-01-02T15:04:05", state["lastupdated"].(string))
							if err != nil {
								panic(err)
							}

							status := tapDialRotatoriesStatuses[uniqueID]

							expectedRotation := state["expectedrotation"].(float64)
							if status.lastUpdate == nil {
								status.lastUpdate = &t
								status.expectedRotation = expectedRotation
							} else {
								if !status.lastUpdate.Equal(t) || expectedRotation != status.expectedRotation {
									status.lastUpdate = &t
									status.expectedRotation = expectedRotation
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

							tapDialRotatoriesStatuses[uniqueID] = status
						}
					}

				}
			}
		} else {
			// groups := state["groups"].(map[string]interface{})
			lights := state["lights"].(map[string]interface{})
			// for _, v := range groups {
			for _, v := range lights {

				// group := v.(map[string]interface{})
				light := v.(map[string]interface{})

				// name := group["name"]
				name := light["name"]
				// if name == "Soggiorno" {
				if name == "Hue Play 5" {

					// state := group["state"].(map[string]interface{})
					state := light["state"].(map[string]interface{})

					// allOn := state["all_on"].(bool)
					allOn := state["on"].(bool)

					if allOn {
						if !goveeOn {

							goveeOn = true

							log.Info().Msgf("Turning on Govee light")
							sendToGovee <- mustMarshal(GoveeTurn{
								Msg: GoveeTurnMsg{
									Cmd: "turn",
									Data: GoveeTurnMsgData{
										Value: 1,
									},
								},
							})
						}
					} else {
						if goveeOn {

							goveeOn = false

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

					// action := group["action"].(map[string]interface{})

					// bri := action["bri"].(float64)
					bri := state["bri"].(float64)
					brightness := int(math.Max(math.Min((bri/255)*100, 100), 0))

					if goveeBrightness != float64(brightness) {

						goveeBrightness = float64(brightness)

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

					// xy := action["xy"].([]interface{})
					xy := state["xy"].([]interface{})

					x := xy[0].(float64)
					y := xy[1].(float64)
					// Y := bri / 255

					r, g, b := xyToRGB(x, y, bri)

					// fmt.Println(x, y, Y*255)

					// r, g, b := colorful.Xyy(x, y, Y).RGB255()
					// k := temperature.ToKelvin(r, g, b)

					// fmt.Println(r, g, b, k)

					if r != goveeColorR || g != goveeColorG || b != goveeColorB /* || k != goveeColorK*/ {
						goveeColorR = r
						goveeColorG = g
						goveeColorB = b
						// goveeColorK = k

						sendToGovee <- mustMarshal(GoveeColorRequest{
							Msg: GoveeColorRequestMsg{
								Cmd: "colorwc",
								Data: GoveeColorRequestMsgData{
									Color: GoveeColorRequestMsgDataColor{
										R: int(r),
										G: int(g),
										B: int(b),
									},
									// Kelvin: int(k),
								},
							},
						})
					}

					// fmt.Println(r, g, b)

				}
			}
		}

		time.Sleep(pollingDuration)
	}

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
				litter.Dump(data)
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
					onOff := data["onOff"].(float64)
					goveeOn = onOff == 1

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

var gamutC = struct {
	r []float64
	g []float64
	b []float64
}{
	r: []float64{0.6915, 0.3083},
	g: []float64{0.17, 0.7},
	b: []float64{0.1532, 0.0475},
}

func xyIsInGamutRange(x, y float64) bool {
	v0 := []float64{gamutC.b[0] - gamutC.r[0], gamutC.b[1] - gamutC.r[1]}
	v1 := []float64{gamutC.g[0] - gamutC.r[0], gamutC.g[1] - gamutC.r[1]}
	v2 := []float64{x - gamutC.r[0], y - gamutC.r[1]}

	dot00 := (v0[0] * v0[0]) + (v0[1] * v0[1])
	dot01 := (v0[0] * v1[0]) + (v0[1] * v1[1])
	dot02 := (v0[0] * v2[0]) + (v0[1] * v2[1])
	dot11 := (v1[0] * v1[0]) + (v1[1] * v1[1])
	dot12 := (v1[0] * v2[0]) + (v1[1] * v2[1])

	invDenom := 1 / (dot00*dot11 - dot01*dot01)

	u := (dot11*dot02 - dot01*dot12) * invDenom
	v := (dot00*dot12 - dot01*dot02) * invDenom

	return u >= 0 && v >= 0 && (u+v < 1)
}

func xyToRGB(x float64, y float64, brightness float64) (uint8, uint8, uint8) {

	if !xyIsInGamutRange(x, y) {
		x, y = getClosestColor(point{x, y})
	}

	z := 1 - x - y
	Y := brightness / 255
	X := (Y / y) * x
	Z := (Y / y) * z

	r := X*1.656492 - Y*0.354851 - Z*0.255038
	g := -X*0.707196 + Y*1.655397 + Z*0.036152
	b := X*0.051713 - Y*0.121364 + Z*1.011530

	if r <= 0.0031308 {
		r = 12.92 * r
	} else {
		r = (1.0+0.055)*math.Pow(r, (1.0/2.4)) - 0.055
	}

	if g <= 0.0031308 {
		g = 12.92 * g
	} else {
		g = (1.0+0.055)*math.Pow(g, (1.0/2.4)) - 0.055
	}

	if b <= 0.0031308 {
		b = 12.92 * b
	} else {
		b = (1.0+0.055)*math.Pow(b, (1.0/2.4)) - 0.055
	}

	// If one component is greater than 1, weight components by that value
	max := math.Max(r, math.Max(g, b))
	if max > 1 {
		r = r / max
		g = g / max
		b = b / max
	}

	return uint8(math.Floor(r * 255)), uint8(math.Floor(g * 255)), uint8(math.Floor(b * 255))
}

type point struct {
	x float64
	y float64
}

func getLineDistance(pointA, pointB point) float64 {
	return math.Hypot(pointB.x-pointA.x, pointB.y-pointA.y)
}

func getClosestPoint(xy, pointA, pointB point) point {
	xy2a := []float64{xy.x - pointA.x, xy.y - pointA.y}
	a2b := []float64{pointB.x - pointA.x, pointB.y - pointA.y}
	a2bSqr := math.Pow(a2b[0], 2) + math.Pow(a2b[1], 2)
	xy2a_dot_a2b := xy2a[0]*a2b[0] + xy2a[1]*a2b[1]
	t := xy2a_dot_a2b / a2bSqr

	return point{
		x: pointA.x + a2b[0]*t,
		y: pointA.y + a2b[1]*t,
	}
}

type ccomb struct {
	a point
	b point
}

type ccombsPoints struct {
	greenBlue point
	greenRed  point
	blueRed   point
}

type ccombsFloats struct {
	greenBlue float64
	greenRed  float64
	blueRed   float64
}

type ccombs struct {
	greenBlue ccombSingle
	greenRed  ccombSingle
	blueRed   ccombSingle
}

type ccombSingle struct {
	closestPoint point
	distance     float64
}

func getClosestColor(xy point) (float64, float64) {
	greenBlue := ccomb{
		a: point{
			x: gamutC.g[0],
			y: gamutC.g[1],
		},
		b: point{
			x: gamutC.b[0],
			y: gamutC.b[1],
		},
	}

	greenRed := ccomb{
		a: point{
			x: gamutC.g[0],
			y: gamutC.g[1],
		},
		b: point{
			x: gamutC.r[0],
			y: gamutC.r[1],
		},
	}

	blueRed := ccomb{
		a: point{
			x: gamutC.r[0],
			y: gamutC.r[1],
		},
		b: point{
			x: gamutC.b[0],
			y: gamutC.b[1],
		},
	}

	closestColorPoints := ccombsPoints{
		greenBlue: getClosestPoint(xy, greenBlue.a, greenBlue.b),
		greenRed:  getClosestPoint(xy, greenRed.a, greenRed.b),
		blueRed:   getClosestPoint(xy, blueRed.a, blueRed.b),
	}

	distance := ccombsFloats{
		greenBlue: getLineDistance(xy, closestColorPoints.greenBlue),
		greenRed:  getLineDistance(xy, closestColorPoints.greenRed),
		blueRed:   getLineDistance(xy, closestColorPoints.blueRed),
	}

	ccombs := ccombs{
		greenBlue: ccombSingle{
			closestPoint: closestColorPoints.greenBlue,
			distance:     distance.greenBlue,
		},
		greenRed: ccombSingle{
			closestPoint: closestColorPoints.greenRed,
			distance:     distance.greenRed,
		},
		blueRed: ccombSingle{
			closestPoint: closestColorPoints.blueRed,
			distance:     distance.blueRed,
		},
	}

	var closestDistance *float64
	var closestColor *point

	for _, c := range []ccombSingle{ccombs.greenBlue, ccombs.greenRed, ccombs.blueRed} {
		if closestDistance == nil {
			closestDistance = valToPtr(c.distance)
			closestColor = valToPtr(c.closestPoint)
		}

		if *closestDistance > c.distance {
			closestDistance = valToPtr(c.distance)
			closestColor = valToPtr(c.closestPoint)
		}
	}

	return closestColor.x, closestColor.y
}

func valToPtr[T any](val T) *T {
	return &val
}
