package utils

import (
	"github.com/stretchr/testify/mock"
)

// MockQueue is a mock implementation of Queue for testing
type MockQueue struct {
	mock.Mock
}

// NewMockQueue creates a new MockQueue instance
func NewMockQueue() *MockQueue {
	return &MockQueue{}
}

// Push mocks the Push method for adding items to queue
func (m *MockQueue) Push(obj any) {
	m.Called(obj)
}

// Pop mocks the Pop method for removing items from queue
func (m *MockQueue) Pop() any {
	args := m.Called()
	return args.Get(0)
}

// Size mocks the Size method for getting queue length
func (m *MockQueue) Size() int {
	args := m.Called()
	return args.Int(0)
}