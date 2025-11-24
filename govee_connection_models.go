package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type FoundGoveeDevice struct {
	IP           string
	SKU          string
	Device       string
	RegisteredAt time.Time
	conn         net.Conn
	connMutex    struct{ sync.Mutex }
	channelOpen  bool
	sendChan     chan []byte
}

type GoveeDeviceStatus struct {
	Brightness float64
	On         bool
}

func (d *FoundGoveeDevice) Send(data []byte) error {
	d.connMutex.Lock()
	defer d.connMutex.Unlock()
	if !d.channelOpen {
		return fmt.Errorf("channel not open")
	}
	d.sendChan <- data
	return nil
}
