package memory

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/ghttp/httpsession"
)

func TestTerminalRemovalPreventsRenewalSave(t *testing.T) {
	for _, removal := range []string{"sweep", "eviction"} {
		t.Run(removal, func(t *testing.T) {
			provider := NewSessionProvider()
			held := provider.NewSession(time.Now().Add(time.Hour))
			if removal == "sweep" {
				held.SetExpire(time.Now().Add(-time.Hour))
				provider.lastClean = time.Now().Add(-time.Minute)
			} else {
				provider.MaxSessions = 1
			}

			current := provider.NewSession(time.Now().Add(time.Hour))
			require.NotContains(t, provider.Sessions(), held.Id(), "the terminal path must remove the old entry")
			held.SetExpire(time.Now().Add(time.Hour))
			require.NoError(t, held.Save())
			require.NoError(t, provider.Save(held))
			require.NotContains(t, provider.Sessions(), held.Id(), "a removed session must not be restored by renewing a held object")
			require.Same(t, current, provider.Session(current.Id()))
			require.Equal(t, int64(len(provider.Sessions())), provider.count.Load())
		})
	}
}

func TestTerminalRemovalRejectsInFlightSave(t *testing.T) {
	for _, removal := range []string{"sweep", "eviction"} {
		t.Run(removal, func(t *testing.T) {
			provider := NewSessionProvider()
			gate := &lifecycleSaveGate{SessionProvider: provider, entered: make(chan struct{}), proceed: make(chan struct{})}
			held := httpsession.NewDefaultSession(gate)
			held.SetExpire(time.Now().Add(time.Hour))
			require.NoError(t, provider.Save(held))
			var wg sync.WaitGroup
			var release sync.Once
			unblock := func() { release.Do(func() { close(gate.proceed) }) }
			defer func() {
				unblock()
				wg.Wait()
			}()
			saved := make(chan error, 1)
			wg.Go(func() { saved <- held.Save() })
			<-gate.entered
			if removal == "sweep" {
				held.SetExpire(time.Now().Add(-time.Hour))
				provider.lastClean = time.Now().Add(-time.Minute)
			} else {
				provider.MaxSessions = 1
			}

			created := make(chan httpsession.Session, 1)
			wg.Go(func() { created <- provider.NewSession(time.Now().Add(time.Hour)) })
			var current httpsession.Session
			select {
			case current = <-created:
			case <-time.After(3 * time.Second):
				t.Fatal("terminal removal waited for an in-flight Save's outer session lock")
			}

			require.NotContains(t, provider.Sessions(), held.Id())
			unblock()
			require.NoError(t, <-saved)
			held.SetExpire(time.Now().Add(time.Hour))
			require.NoError(t, provider.Save(held))
			require.NotContains(t, provider.Sessions(), held.Id(), "a Save started before revocation must not publish afterward")
			require.Same(t, current, provider.Session(current.Id()))
			require.Equal(t, int64(len(provider.Sessions())), provider.count.Load())
		})
	}
}

type lifecycleSaveGate struct {
	*SessionProvider
	entered chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (g *lifecycleSaveGate) Save(session httpsession.Session) (rtErr error) {
	g.once.Do(func() {
		close(g.entered)
		<-g.proceed
	})
	return g.SessionProvider.Save(session)
}

func TestEvictionPreservesRenewalAfterScan(t *testing.T) {
	for _, renewal := range []string{"renew", "renew-save"} {
		t.Run(renewal, func(t *testing.T) {
			provider := NewSessionProvider()
			held := &lifecycleScanSession{DefaultSession: httpsession.NewDefaultSession(provider), entered: make(chan struct{}), proceed: make(chan struct{})}
			held.SetExpire(time.Now().Add(time.Hour))
			require.NoError(t, provider.Save(held))
			held.SetExpire(time.Now().Add(-time.Hour))
			provider.MaxSessions = 1
			var wg sync.WaitGroup
			var release sync.Once
			unblock := func() { release.Do(func() { close(held.proceed) }) }
			defer func() {
				unblock()
				wg.Wait()
			}()
			created := make(chan httpsession.Session, 1)
			wg.Go(func() { created <- provider.NewSession(time.Now().Add(time.Hour)) })
			<-held.entered
			held.SetExpire(time.Now().Add(time.Hour))
			if renewal == "renew-save" {
				require.NoError(t, held.Save())
			}

			unblock()
			require.NotNil(t, <-created)
			require.False(t, held.IsExpire(), "a stale eviction scan must not revoke a renewed object")
			require.NotNil(t, provider.Session(held.Id()), "renewal or a new Save generation must survive the stale scan")
			require.Equal(t, int64(len(provider.Sessions())), provider.count.Load())
		})
	}
}

type lifecycleScanSession struct {
	*httpsession.DefaultSession
	entered chan struct{}
	proceed chan struct{}
	once    sync.Once
}

func (s *lifecycleScanSession) Updated() (updated time.Time) {
	s.once.Do(func() {
		close(s.entered)
		<-s.proceed
	})
	return s.DefaultSession.Updated()
}

func TestDeletePreservesReplacementObject(t *testing.T) {
	provider := NewSessionProvider()
	held := &lifecycleDeleteGate{DefaultSession: httpsession.NewDefaultSession(provider), entered: make(chan struct{}), proceed: make(chan struct{})}
	held.SetExpire(time.Now().Add(time.Hour))
	require.NoError(t, provider.Save(held))
	data, err := json.Marshal(held)
	require.NoError(t, err)
	replacement := httpsession.NewDefaultSession(provider)
	require.NoError(t, json.Unmarshal(data, replacement))
	replacement.PutString("generation", "new")
	var wg sync.WaitGroup
	var release sync.Once
	unblock := func() { release.Do(func() { close(held.proceed) }) }
	defer func() {
		unblock()
		wg.Wait()
	}()
	wg.Go(func() { provider.Delete(held.Id()) })
	<-held.entered
	require.NoError(t, provider.Save(replacement))
	unblock()
	wg.Wait()
	require.True(t, held.IsExpire(), "Delete must revoke its observed object")
	require.False(t, replacement.IsExpire(), "Delete must not revoke a separately published replacement")
	require.Same(t, replacement, provider.Session(replacement.Id()))
	require.Equal(t, int64(len(provider.Sessions())), provider.count.Load())
}

type lifecycleDeleteGate struct {
	*httpsession.DefaultSession
	entered chan struct{}
	proceed chan struct{}
}

func (s *lifecycleDeleteGate) MarkDeleted() {
	close(s.entered)
	<-s.proceed
	s.DefaultSession.MarkDeleted()
}
