package httpsession

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExpirationAtCurrentSecond(t *testing.T) {
	session := NewDefaultSession(NewMockSessionProvider())
	session.SetExpire(time.Now())
	require.True(t, session.IsExpire(), "the expiry second is excluded from session lifetime")
	session.SetExpire(time.Now().Add(time.Hour))
	require.False(t, session.IsExpire())
}
