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
	x          float64
	y          float64
}
