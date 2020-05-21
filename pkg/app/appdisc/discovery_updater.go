package appdisc

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/SkycoinProject/skywire-mainnet/pkg/proxydisc"
)

// Updater updates the associated app discovery
type Updater interface {

	// Start starts the updater.
	Start()

	// Stop stops the updater.
	Stop()

	// ChangeValue changes the associated value of the discovery entry.
	ChangeValue(name string, v []byte) error
}

// emptyUpdater is for apps that do not require discovery updates.
type emptyUpdater struct{}

func (emptyUpdater) Start()                                  {}
func (emptyUpdater) Stop()                                   {}
func (emptyUpdater) ChangeValue(name string, v []byte) error { return nil }

// proxyUpdater updates proxy-discovery entry of locally running skysocks App.
type proxyUpdater struct {
	client   *proxydisc.HTTPClient
	interval time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

func (u *proxyUpdater) Start() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel

	u.wg.Add(1)
	go func() {
		u.client.UpdateLoop(ctx, u.interval)
		u.wg.Done()
	}()
}

func (u *proxyUpdater) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.cancel == nil {
		return
	}

	u.cancel()
	u.cancel = nil
	u.wg.Wait()
}

func (u *proxyUpdater) ChangeValue(name string, v []byte) error {
	switch name {
	case ConnCountValue:
		n, err := strconv.Atoi(string(v))
		if err != nil {
			return err
		}
		go u.client.UpdateStats(proxydisc.Stats{ConnectedClients: n})
	}
	return nil
}