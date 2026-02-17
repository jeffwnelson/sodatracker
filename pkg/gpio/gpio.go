package gpio

// Pin represents a GPIO pin that can be read.
type Pin interface {
	Read() (bool, error)
	Number() int
}

// Provider creates Pin instances for given pin numbers.
type Provider interface {
	OpenInput(pin int) (Pin, error)
	Close() error
}
