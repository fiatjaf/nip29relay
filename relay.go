package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fiatjaf/relayer/v2"
	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
)

type Relay struct {
	storage *lmdbchatbackend
}

func (r *Relay) Name() string {
	return "GroupsRelay"
}

func (r *Relay) Storage(ctx context.Context) relayer.Storage {
	return r.storage
}

func (r *Relay) OnInitialized(*relayer.Server) {}

func (r *Relay) Init() error {
	err := envconfig.Process("", r)
	if err != nil {
		return fmt.Errorf("couldn't process envconfig: %w", err)
	}

	// every hour, delete all very old events
	go func() {
		// db := r.Storage().(lmdbchatbackend)

		for {
			time.Sleep(60 * time.Minute)
			// db.DB.Exec(`DELETE FROM event WHERE created_at < $1`, time.Now().AddDate(0, -3, 0).Unix()) // 3 months
		}
	}()

	return nil
}

func (r *Relay) AcceptEvent(ctx context.Context, evt *nostr.Event) bool {
	// only accept kind 9
	if evt.Kind != 9 {
		return false
	}

	// block events that are too large
	jsonb, _ := json.Marshal(evt)
	if len(jsonb) > 10000 {
		return false
	}

	return true
}
