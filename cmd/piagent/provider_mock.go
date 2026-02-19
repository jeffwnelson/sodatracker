//go:build !rpi

package main

import "github.com/jeffwnelson/sodatracker/pkg/gpio"

func newProvider() (gpio.Provider, error) {
	return gpio.NewMockProvider(), nil
}
