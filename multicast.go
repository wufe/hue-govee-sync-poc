package main

import (
	"encoding/json"
	"fmt"
	"net"
)

const (
	multicastAddress = "239.255.255.250"
	broadcastPort    = 4001
)

var multicastAddr *net.UDPAddr

func init() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", multicastAddress, broadcastPort))
	if err != nil {
		panic(fmt.Errorf("error resolving UDP address: %w", err))
	}
	multicastAddr = addr
}

func openMulticastConnection() (*net.UDPConn, error) {
	conn, err := net.ListenMulticastUDP("udp", nil, multicastAddr)
	if err != nil {
		return nil, fmt.Errorf("error listening to multicast UDP: %v", err)
	}

	return conn, nil
}

func sendScanRequest(conn *net.UDPConn) error {
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
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	_, err = conn.WriteTo(requestJSON, multicastAddr)
	if err != nil {
		return fmt.Errorf("error sending 'request scan' message: %v", err)
	}

	return nil
}
