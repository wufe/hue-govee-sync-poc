package main

import "time"

type DialStatus struct {
	lastUpdate  *time.Time
	buttonEvent float64
}

type DialRotaryStatus struct {
	lastUpdate       *time.Time
	expectedRotation float64
}

type LightStatus struct {
	lastUpdate *time.Time
	on         bool
	brightness int
	r          uint8
	g          uint8
	b          uint8
}

func (s LightStatus) EqualsOn(other LightStatus) bool {
	return s.on == other.on
}

func (s LightStatus) EqualsBrightness(other LightStatus) bool {
	return s.brightness == other.brightness
}

func (s LightStatus) EqualsColor(other LightStatus) bool {
	return s.r == other.r && s.g == other.g && s.b == other.b
}

type PresenceSensorStatus struct {
	presence    bool
	lastUpdated *time.Time
}
