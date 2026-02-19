//go:build rpi

package gpio

import (
	"fmt"

	periphgpio "periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
)

// RPiProvider reads real GPIO pins via periph.io.
type RPiProvider struct{}

func NewRPiProvider() (*RPiProvider, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("periph init: %w", err)
	}
	return &RPiProvider{}, nil
}

func (p *RPiProvider) OpenInput(pin int) (Pin, error) {
	name := fmt.Sprintf("GPIO%d", pin)
	gpin := gpioreg.ByName(name)
	if gpin == nil {
		return nil, fmt.Errorf("pin %s not found", name)
	}
	if err := gpin.In(periphgpio.PullUp, periphgpio.NoEdge); err != nil {
		return nil, fmt.Errorf("configure pin %s as input with pull-up: %w", name, err)
	}
	return &RPiPin{pin: gpin, number: pin}, nil
}

func (p *RPiProvider) Close() error { return nil }

type RPiPin struct {
	pin    periphgpio.PinIO
	number int
}

func (p *RPiPin) Read() (bool, error) {
	return p.pin.Read() == periphgpio.High, nil
}

func (p *RPiPin) Number() int { return p.number }
