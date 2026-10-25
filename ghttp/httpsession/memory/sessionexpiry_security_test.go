package memory

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExpiredLookupAndStaleSave(t *testing.T) {
	provider := NewSessionProvider()
	session := provider.NewSession(time.Now().Add(time.Hour))
	session.SetExpire(time.Now().Add(-time.Hour))
	require.Nil(t, provider.Session(session.Id()), "lookup must reject expired state without waiting for sweep")
	require.NotContains(t, provider.Sessions(), session.Id())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				if err := session.Save(); err != nil {
					t.Errorf("saving a held session: %v", err)
				}

				if err := provider.Save(session); err != nil {
					t.Errorf("saving through provider: %v", err)
				}

				if provider.Session(session.Id()) != nil {
					t.Error("concurrent Save must not resurrect an expired session")
				}
			}
		})
	}

	wg.Wait()
	require.NotContains(t, provider.Sessions(), session.Id(), "held expired state must not be resurrected")
	valid := provider.NewSession(time.Now().Add(time.Hour))
	require.Same(t, valid, provider.Session(valid.Id()))
}
