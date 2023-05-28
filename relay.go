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

func (r *Relay) Name() string {
	return "n29"
}

func (r *Relay) Storage(ctx context.Context) relayer.Storage {
	return r.storage
}

func (r *Relay) Init() error {
	return nil
}

func (r *Relay) AcceptEvent(ctx context.Context, evt *nostr.Event) bool {
	// only accept nip29 events for now -- later we could also store kind:0
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
		Description:   "relay specialized in public chat groups",
		SupportedNIPs: []int{29},
		Contact:       "nostr:npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
		Software:      "git@github.com:fiatjaf/nip29relay.git",
		Version:       "pre-alpha",
	}
}
