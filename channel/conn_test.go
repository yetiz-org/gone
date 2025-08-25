package channel

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockNetConn is a mock implementation of net.Conn for testing
type MockNetConn struct {
	mock.Mock
}

func (m *MockNetConn) Read(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *MockNetConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *MockNetConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNetConn) LocalAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *MockNetConn) RemoteAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *MockNetConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockNetConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockNetConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

// TestWrapConn tests the connection wrapper function
func TestWrapConn(t *testing.T) {
	t.Parallel()

	t.Run("WrapConn_ValidConnection", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}

		conn := WrapConn(mockNetConn)

		// Should return non-nil wrapped connection
		assert.NotNil(t, conn)

		// Should be DefaultConn implementation
		defaultConn, ok := conn.(*DefaultConn)
		assert.True(t, ok)
		assert.NotNil(t, defaultConn)

		// Should be active by default
		assert.True(t, conn.IsActive())

		// Should wrap the original connection
		assert.Equal(t, mockNetConn, conn.Conn())
	})

	t.Run("WrapConn_NilConnection", func(t *testing.T) {
		t.Parallel()

		conn := WrapConn(nil)

		// Should return nil for nil input
		assert.Nil(t, conn)
	})
}

// TestDefaultConn_IsActive tests the active state functionality
func TestDefaultConn_IsActive(t *testing.T) {
	t.Parallel()

	t.Run("IsActive_InitiallyTrue", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		conn := WrapConn(mockNetConn)

		// Should be active initially
		assert.True(t, conn.IsActive())
	})

	t.Run("IsActive_AfterClose", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		mockNetConn.On("Close").Return(nil)

		conn := WrapConn(mockNetConn)

		// Close the connection
		err := conn.Close()
		assert.NoError(t, err)

		// Should be inactive after close
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})
}

// TestDefaultConn_Conn tests the underlying connection getter
func TestDefaultConn_Conn(t *testing.T) {
	t.Parallel()

	t.Run("Conn_ReturnsUnderlyingConnection", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		conn := WrapConn(mockNetConn)

		// Should return the wrapped connection
		underlying := conn.Conn()
		assert.Equal(t, mockNetConn, underlying)
	})
}

// TestDefaultConn_Read tests the read functionality
func TestDefaultConn_Read(t *testing.T) {
	t.Parallel()

	t.Run("Read_SuccessfulRead", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		expectedBytes := 9

		mockNetConn.On("Read", mock.AnythingOfType("[]uint8")).Return(expectedBytes, nil)

		conn := WrapConn(mockNetConn)
		buffer := make([]byte, 100)

		n, err := conn.Read(buffer)

		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, expectedBytes, n)

		// Should still be active
		assert.True(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Read_ErrorWithDeadlineExceeded", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		expectedBytes := 5
		deadlineErr := os.ErrDeadlineExceeded

		mockNetConn.On("Read", mock.AnythingOfType("[]uint8")).Return(expectedBytes, deadlineErr)

		conn := WrapConn(mockNetConn)
		buffer := make([]byte, 100)

		n, err := conn.Read(buffer)

		// Should return the error but not deactivate
		assert.Error(t, err)
		assert.Equal(t, deadlineErr, err)
		assert.Equal(t, expectedBytes, n)

		// Should still be active for deadline exceeded
		assert.True(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Read_ErrorDeactivatesConnection", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		expectedBytes := 0
		readErr := errors.New("connection closed")

		mockNetConn.On("Read", mock.AnythingOfType("[]uint8")).Return(expectedBytes, readErr)

		conn := WrapConn(mockNetConn)
		buffer := make([]byte, 100)

		n, err := conn.Read(buffer)

		// Should return the error and deactivate
		assert.Error(t, err)
		assert.Equal(t, readErr, err)
		assert.Equal(t, expectedBytes, n)

		// Should be deactivated
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Read_ErrorOnInactiveConnection", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		readErr := errors.New("connection error")

		mockNetConn.On("Read", mock.AnythingOfType("[]uint8")).Return(0, readErr)
		mockNetConn.On("Close").Return(nil)

		conn := WrapConn(mockNetConn)

		// First close the connection to make it inactive
		conn.Close()
		assert.False(t, conn.IsActive())

		// Read should still return error but not change active state
		buffer := make([]byte, 100)
		n, err := conn.Read(buffer)

		assert.Error(t, err)
		assert.Equal(t, 0, n)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})
}

// TestDefaultConn_Write tests the write functionality
func TestDefaultConn_Write(t *testing.T) {
	t.Parallel()

	t.Run("Write_SuccessfulWrite", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		testData := []byte("test data")
		expectedBytes := len(testData)

		mockNetConn.On("Write", testData).Return(expectedBytes, nil)

		conn := WrapConn(mockNetConn)

		n, err := conn.Write(testData)

		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, expectedBytes, n)

		// Should still be active
		assert.True(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Write_ErrorWithDeadlineExceeded", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		testData := []byte("test data")
		expectedBytes := 5
		deadlineErr := os.ErrDeadlineExceeded

		mockNetConn.On("Write", testData).Return(expectedBytes, deadlineErr)

		conn := WrapConn(mockNetConn)

		n, err := conn.Write(testData)

		// Should return the error but not deactivate
		assert.Error(t, err)
		assert.Equal(t, deadlineErr, err)
		assert.Equal(t, expectedBytes, n)

		// Should still be active for deadline exceeded
		assert.True(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Write_ErrorDeactivatesConnection", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		testData := []byte("test data")
		expectedBytes := 0
		writeErr := errors.New("connection closed")

		mockNetConn.On("Write", testData).Return(expectedBytes, writeErr)

		conn := WrapConn(mockNetConn)

		n, err := conn.Write(testData)

		// Should return the error and deactivate
		assert.Error(t, err)
		assert.Equal(t, writeErr, err)
		assert.Equal(t, expectedBytes, n)

		// Should be deactivated
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Write_ErrorOnInactiveConnection", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		testData := []byte("test data")
		writeErr := errors.New("connection error")

		mockNetConn.On("Write", testData).Return(0, writeErr)
		mockNetConn.On("Close").Return(nil)

		conn := WrapConn(mockNetConn)

		// First close the connection to make it inactive
		conn.Close()
		assert.False(t, conn.IsActive())

		// Write should still return error but not change active state
		n, err := conn.Write(testData)

		assert.Error(t, err)
		assert.Equal(t, 0, n)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})
}

// TestDefaultConn_Close tests the close functionality
func TestDefaultConn_Close(t *testing.T) {
	t.Parallel()

	t.Run("Close_SuccessfulClose", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		mockNetConn.On("Close").Return(nil)

		conn := WrapConn(mockNetConn)

		// Should be active initially
		assert.True(t, conn.IsActive())

		err := conn.Close()

		// Should succeed and deactivate
		assert.NoError(t, err)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Close_ErrorOnClose", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		closeErr := errors.New("close error")
		mockNetConn.On("Close").Return(closeErr)

		conn := WrapConn(mockNetConn)

		err := conn.Close()

		// Should return error but still deactivate
		assert.Error(t, err)
		assert.Equal(t, closeErr, err)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Close_MultipleCloses", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		mockNetConn.On("Close").Return(nil).Times(2)

		conn := WrapConn(mockNetConn)

		// First close
		err1 := conn.Close()
		assert.NoError(t, err1)
		assert.False(t, conn.IsActive())

		// Second close should still work
		err2 := conn.Close()
		assert.NoError(t, err2)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})
}

// TestDefaultConn_AddressMethods tests address getter methods
func TestDefaultConn_AddressMethods(t *testing.T) {
	t.Parallel()

	t.Run("LocalAddr_ReturnsCorrectAddress", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		mockNetConn.On("LocalAddr").Return(localAddr)

		conn := WrapConn(mockNetConn)

		addr := conn.LocalAddr()

		assert.Equal(t, localAddr, addr)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("LocalAddr_ReturnsNil", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		mockNetConn.On("LocalAddr").Return(nil)

		conn := WrapConn(mockNetConn)

		addr := conn.LocalAddr()

		assert.Nil(t, addr)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("RemoteAddr_ReturnsCorrectAddress", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8081}
		mockNetConn.On("RemoteAddr").Return(remoteAddr)

		conn := WrapConn(mockNetConn)

		addr := conn.RemoteAddr()

		assert.Equal(t, remoteAddr, addr)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("RemoteAddr_ReturnsNil", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		mockNetConn.On("RemoteAddr").Return(nil)

		conn := WrapConn(mockNetConn)

		addr := conn.RemoteAddr()

		assert.Nil(t, addr)
		mockNetConn.AssertExpectations(t)
	})
}

// TestDefaultConn_DeadlineMethods tests deadline setting methods
func TestDefaultConn_DeadlineMethods(t *testing.T) {
	t.Parallel()

	t.Run("SetDeadline_Success", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		deadline := time.Now().Add(5 * time.Second)
		mockNetConn.On("SetDeadline", deadline).Return(nil)

		conn := WrapConn(mockNetConn)

		err := conn.SetDeadline(deadline)

		assert.NoError(t, err)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("SetDeadline_Error", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		deadline := time.Now().Add(5 * time.Second)
		deadlineErr := errors.New("deadline error")
		mockNetConn.On("SetDeadline", deadline).Return(deadlineErr)

		conn := WrapConn(mockNetConn)

		err := conn.SetDeadline(deadline)

		assert.Error(t, err)
		assert.Equal(t, deadlineErr, err)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("SetReadDeadline_Success", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		deadline := time.Now().Add(3 * time.Second)
		mockNetConn.On("SetReadDeadline", deadline).Return(nil)

		conn := WrapConn(mockNetConn)

		err := conn.SetReadDeadline(deadline)

		assert.NoError(t, err)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("SetReadDeadline_Error", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		deadline := time.Now().Add(3 * time.Second)
		readDeadlineErr := errors.New("read deadline error")
		mockNetConn.On("SetReadDeadline", deadline).Return(readDeadlineErr)

		conn := WrapConn(mockNetConn)

		err := conn.SetReadDeadline(deadline)

		assert.Error(t, err)
		assert.Equal(t, readDeadlineErr, err)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("SetWriteDeadline_Success", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		deadline := time.Now().Add(7 * time.Second)
		mockNetConn.On("SetWriteDeadline", deadline).Return(nil)

		conn := WrapConn(mockNetConn)

		err := conn.SetWriteDeadline(deadline)

		assert.NoError(t, err)
		mockNetConn.AssertExpectations(t)
	})

	t.Run("SetWriteDeadline_Error", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}
		deadline := time.Now().Add(7 * time.Second)
		writeDeadlineErr := errors.New("write deadline error")
		mockNetConn.On("SetWriteDeadline", deadline).Return(writeDeadlineErr)

		conn := WrapConn(mockNetConn)

		err := conn.SetWriteDeadline(deadline)

		assert.Error(t, err)
		assert.Equal(t, writeDeadlineErr, err)
		mockNetConn.AssertExpectations(t)
	})
}

// TestDefaultConn_IntegrationScenarios tests complete connection workflows
func TestDefaultConn_IntegrationScenarios(t *testing.T) {
	t.Parallel()

	t.Run("Conn_CompleteReadWriteLifecycle", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}

		// Setup expectations for complete lifecycle
		testData := []byte("hello world")
		mockNetConn.On("Write", testData).Return(len(testData), nil)
		mockNetConn.On("Read", mock.AnythingOfType("[]uint8")).Return(len(testData), nil)
		mockNetConn.On("Close").Return(nil)

		conn := WrapConn(mockNetConn)

		// Should be active initially
		assert.True(t, conn.IsActive())

		// Write data
		n, err := conn.Write(testData)
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.True(t, conn.IsActive())

		// Read data
		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.True(t, conn.IsActive())

		// Close connection
		err = conn.Close()
		assert.NoError(t, err)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Conn_ErrorRecoveryScenarios", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}

		// First write succeeds
		testData1 := []byte("first write")
		mockNetConn.On("Write", testData1).Return(len(testData1), nil)

		// Second write fails with non-deadline error (should deactivate)
		testData2 := []byte("second write")
		writeErr := errors.New("network error")
		mockNetConn.On("Write", testData2).Return(0, writeErr)

		// Third write on inactive connection
		testData3 := []byte("third write")
		mockNetConn.On("Write", testData3).Return(0, writeErr)

		conn := WrapConn(mockNetConn)

		// First write should succeed
		n, err := conn.Write(testData1)
		assert.NoError(t, err)
		assert.Equal(t, len(testData1), n)
		assert.True(t, conn.IsActive())

		// Second write should fail and deactivate
		n, err = conn.Write(testData2)
		assert.Error(t, err)
		assert.Equal(t, writeErr, err)
		assert.Equal(t, 0, n)
		assert.False(t, conn.IsActive())

		// Third write should still fail (connection inactive)
		n, err = conn.Write(testData3)
		assert.Error(t, err)
		assert.Equal(t, 0, n)
		assert.False(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})

	t.Run("Conn_ConcurrentOperations", func(t *testing.T) {
		t.Parallel()

		mockNetConn := &MockNetConn{}

		// Setup expectations for concurrent operations
		testData := []byte("concurrent test")
		mockNetConn.On("Write", testData).Return(len(testData), nil).Times(10)
		mockNetConn.On("Read", mock.AnythingOfType("[]uint8")).Return(len(testData), nil).Times(10)

		conn := WrapConn(mockNetConn)

		// Run concurrent writes and reads
		done := make(chan bool, 20)

		// Concurrent writes
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				n, err := conn.Write(testData)
				assert.NoError(t, err)
				assert.Equal(t, len(testData), n)
			}()
		}

		// Concurrent reads
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				buffer := make([]byte, 100)
				n, err := conn.Read(buffer)
				assert.NoError(t, err)
				assert.Equal(t, len(testData), n)
			}()
		}

		// Wait for all operations to complete
		for i := 0; i < 20; i++ {
			<-done
		}

		// Connection should still be active
		assert.True(t, conn.IsActive())

		mockNetConn.AssertExpectations(t)
	})
}
