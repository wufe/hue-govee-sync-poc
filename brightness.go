package main

import (
	"sync"

	"golang.org/x/sync/singleflight"
)

type BrightnessRetriever interface {
	GetDeviceBrightness(identifier string) (int, error)
}

var (
	Brightness = NewBrightnessManager()
)

type BrightnessManager struct {
	cache sync.Map
	sf    singleflight.Group
}

func NewBrightnessManager() *BrightnessManager {
	return &BrightnessManager{}
}

func (b *BrightnessManager) SetForDevice(deviceName string, val int) {
	b.cache.Store(deviceName, val)
}

type getDeviceBrightnessOptions struct {
	onMissingBrightness func(deviceName string) (int, error)
}

type getDeviceBrightnessOption func(*getDeviceBrightnessOptions)

func WithOnMissingBrightness(f func(deviceName string) (int, error)) getDeviceBrightnessOption {
	return func(opts *getDeviceBrightnessOptions) {
		opts.onMissingBrightness = f
	}
}

func (b *BrightnessManager) GetDeviceBrightness(deviceName string, opts ...getDeviceBrightnessOption) int {

	options := &getDeviceBrightnessOptions{}

	for _, opt := range opts {
		opt(options)
	}

	v, _, _ := b.sf.Do(deviceName, func() (interface{}, error) {
		val, found := b.cache.Load(deviceName)
		if found {
			return val, nil
		}

		if options.onMissingBrightness == nil {
			return 50, nil
		}

		val, err := options.onMissingBrightness(deviceName)
		if err != nil {
			return 50, nil
		}

		b.cache.Store(deviceName, val)

		return val, nil
	})
	return v.(int)
}
