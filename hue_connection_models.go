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
