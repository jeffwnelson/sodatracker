package gpio

import "sync"

// MockProvider is a GPIO provider for testing that allows programmatic pin control.
type MockProvider struct {
	mu   sync.Mutex
	pins map[int]*MockPin
}

func NewMockProvider() *MockProvider {
	return &MockProvider{pins: make(map[int]*MockPin)}
}

func (m *MockProvider) OpenInput(pin int) (Pin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := &MockPin{number: pin, high: true} // default: not pressed (pull-up)
	m.pins[pin] = p
	return p, nil
}

func (m *MockProvider) Close() error { return nil }

// SetPin sets the value of a mock pin (false = pressed for active-low buttons).
func (m *MockProvider) SetPin(pin int, high bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.pins[pin]; ok {
		p.mu.Lock()
		p.high = high
		p.mu.Unlock()
	}
}

type MockPin struct {
	number int
	mu     sync.Mutex
	high   bool
}

func (p *MockPin) Read() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.high, nil
}

func (p *MockPin) Number() int { return p.number }
