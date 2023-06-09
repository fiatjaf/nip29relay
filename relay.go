package main

import (
	"context"
	"encoding/json"

	"github.com/fiatjaf/relayer/v2"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
)

type Relay struct {
	storage *lmdbchatbackend
}

var (
	_ relayer.Relay         = (*Relay)(nil)
	_ relayer.Informationer = (*Relay)(nil)
	_ relayer.Auther        = (*Relay)(nil)
)

func (r *Relay) Name() string {
	return "n29"
}

func (r *Relay) Storage(ctx context.Context) relayer.Storage {
	return r.storage
}

func (r *Relay) Init() error {
	return nil
}

func (r *Relay) ServiceURL() string {
	return "wss://" + config.PublicHostname
}

func (r *Relay) AcceptEvent(ctx context.Context, evt *nostr.Event) bool {
	// only accept nip29 events
	if evt.Kind == nostr.KindSimpleChatMessage || evt.Kind == nostr.KindSimpleChatAction {
		gtags := evt.Tags.GetAll([]string{"g", "/"})
		if len(gtags) == 0 || len(gtags) > 2 {
			return false
		}
	} else {
		return false
	}

	// block events that are too large
	jsonb, _ := json.Marshal(evt)
	if len(jsonb) > 10000 {
		return false
	}

	// if it's an action, check permission and block otherwise
	// TODO

	return true
}

func (r *Relay) GetNIP11InformationDocument() nip11.RelayInformationDocument {
	pubkey, _ := nostr.GetPublicKey(config.PrivateKey)
	return nip11.RelayInformationDocument{
		Name:          "n29",
		PubKey:        pubkey,
		Description:   config.Description,
		SupportedNIPs: []int{29},
		Software:      "git@github.com:fiatjaf/nip29relay.git",
		Version:       "pre-alpha",
	}
}
