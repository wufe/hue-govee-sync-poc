package main

import (
	"maps"
	"sync"
)

type deviceStatus struct {
	Provider   string      `json:"provider"`
	On         int         `json:"on"` // -1=unknown, 0=off, 1=on
	Brightness int         `json:"brightness"`
	Color      deviceColor `json:"color"`
}

type deviceColor struct {
	Red   int `json:"r"`
	Green int `json:"g"`
	Blue  int `json:"b"`
}

func getDefaultDeviceStatus() deviceStatus {
	return deviceStatus{
		Brightness: -1,
		On:         -1,
		Color: deviceColor{
			Red:   -1,
			Green: -1,
			Blue:  -1,
		},
	}
}

type status struct {
	mtx      struct{ sync.RWMutex }
	statuses map[string]deviceStatus
}

var Status = &status{
	statuses: make(map[string]deviceStatus),
}

func (s *status) Register(device, provider string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	_, ok := s.statuses[device]
	if !ok {
		deviceStatus := getDefaultDeviceStatus()
		deviceStatus.Provider = provider
		s.statuses[device] = deviceStatus
	}
}

func (s *status) SetBrightness(device string, brightness int) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	ds, ok := s.statuses[device]
	if !ok {
		ds = getDefaultDeviceStatus()
		s.statuses[device] = ds
	}
	ds.Brightness = brightness
	s.statuses[device] = ds
}

func (s *status) SetOn(device string, on bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	ds, ok := s.statuses[device]
	if !ok {
		ds = getDefaultDeviceStatus()
		s.statuses[device] = ds
	}
	if on {
		ds.On = 1
	} else {
		ds.On = 0
	}
	s.statuses[device] = ds
}

func (s *status) SetColor(device string, red, green, blue int) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	ds, ok := s.statuses[device]
	if !ok {
		ds = getDefaultDeviceStatus()
		s.statuses[device] = ds
	}
	ds.Color.Red = red
	ds.Color.Green = green
	ds.Color.Blue = blue
	s.statuses[device] = ds
}

func (s *status) GetAll() map[string]deviceStatus {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	copied := make(map[string]deviceStatus)
	maps.Copy(copied, s.statuses)
	return copied
}
