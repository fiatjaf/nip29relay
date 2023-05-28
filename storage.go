package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/mailru/easyjson"
	"github.com/nbd-wtf/go-nostr"
	"golang.org/x/exp/slices"
)

var serial uint32

type lmdbchatbackend struct {
	lmdbPath string
	lmdbEnv  *lmdb.Env

	// events are indexed by their kind and then created_at timestamp only, then a serial integer for order consistency
	groups map[string]*lmdb.DBI
	mutex  sync.Mutex
}

func (db *lmdbchatbackend) Init() error {
	// initialize lmdb
	env, err := lmdb.NewEnv()
	if err != nil {
		return err
	}

	env.SetMaxDBs(256) // max number of rooms
	env.SetMaxReaders(500)
	env.SetMapSize(1 << 38) // ~273GB

	err = env.Open(db.lmdbPath, lmdb.NoTLS, 0644)
	if err != nil {
		return err
	}
	db.lmdbEnv = env

	// create channels map of dbis
	db.groups = make(map[string]*lmdb.DBI)

	return nil
}

func (db *lmdbchatbackend) getGroup(id string) (*lmdb.DBI, error) {
	if !slices.Contains(config.Groups, id) {
		return nil, fmt.Errorf("group '%s' not allowed", id)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()
	if channel, ok := db.groups[id]; ok {
		return channel, nil
	}

	// not opened yet, so open and store it
	var channel *lmdb.DBI
	if err := db.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		if dbi, err := txn.OpenDBI(id, lmdb.Create); err != nil {
			return err
		} else {
			channel = &dbi
			db.groups[id] = &dbi
			return nil
		}
	}); err != nil {
		return nil, err
	}

	return channel, nil
}

func (db *lmdbchatbackend) QueryEvents(ctx context.Context, filter *nostr.Filter) (ch chan *nostr.Event, err error) {
	ch = make(chan *nostr.Event)

	type query struct {
		kind  int
		group string
	}
	queries := make([]query, 0, len(filter.Kinds))

	for _, kind := range filter.Kinds {
		var tagName string
		switch kind {
		case nostr.KindSimpleChatMessage, nostr.KindSimpleChatAction:
			tagName = "g"
		case nostr.KindSimpleChatMetadata, nostr.KindSimpleChatPermissions:
			tagName = "d"
		default:
			continue
		}
		if gtags, ok := filter.Tags[tagName]; ok {
			for _, tag := range gtags {
				if !strings.HasPrefix(tag, "/") {
					continue
				}
				queries = append(queries, query{kind, tag})
			}
		}
	}

	if len(queries) == 0 {
		return ch, fmt.Errorf("must pick a group from where to read")
	}

	var until uint32 = uint32(nostr.Now())
	if filter.Until != nil {
		until = uint32(*filter.Until)
	}
	var since uint32 = 0
	if filter.Since != nil {
		since = uint32(*filter.Since)
	}
	limit := 200
	if filter.Limit != 0 && filter.Limit < 300 {
		limit = filter.Limit
	}

	wg := sync.WaitGroup{}
	wg.Add(len(queries))
	for _, q := range queries {
		go func(q query) {
			defer wg.Done()

			if group, err := db.getGroup(q.group); err != nil {
				log.Error().Err(err).Str("group", q.group).Msg("failed to get channel")
				return
			} else {
				db.lmdbEnv.View(func(txn *lmdb.Txn) error {
					txn.RawRead = true

					cursor, err := txn.OpenCursor(*group)
					if err != nil {
						log.Error().Err(err).Str("group", q.group).Msg("failed to open cursor")
						return err
					}
					defer cursor.Close()

					prefix := make([]byte, 6)
					binary.BigEndian.PutUint16(prefix[0:2], uint16(q.kind))
					binary.BigEndian.PutUint32(prefix[2:6], until)
					nextKey := prefix

					i := 0
					for {
						// exit early if the context was canceled
						select {
						case <-ctx.Done():
							break
						default:
						}

						nextKey, v, err := cursor.Get(nextKey, nil, lmdb.PrevNoDup)
						if err != nil {
							break
						}

						if ts := binary.BigEndian.Uint32(nextKey[0:4]); ts < since {
							break
						}

						var evt nostr.Event
						if err := json.Unmarshal(v, &evt); err != nil {
							continue
						}

						ch <- &evt
						i++
						if i == limit {
							break
						}
					}

					return nil
				})
			}
		}(q)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch, nil
}

func (db *lmdbchatbackend) DeleteEvent(ctx context.Context, id string, pubkey string) error {
	return fmt.Errorf("delete functionality not implemented")
}

func (db *lmdbchatbackend) SaveEvent(ctx context.Context, event *nostr.Event) error {
	// since we have filtered out everything else on AcceptEvent, here we know we'll get either a message or an action
	gtags := event.Tags.GetAll([]string{"g", "/"})
	for _, gtag := range gtags {
		if len(gtag) < 2 {
			continue
		}

		if group, err := db.getGroup(gtag[1]); err != nil {
			return fmt.Errorf("failed to open channel db: %w", err)
		} else {
			switch event.Kind {
			// it's a message, store it
			case nostr.KindSimpleChatMessage:
				key := make([]byte, 10)
				binary.BigEndian.PutUint16(key[0:2], uint16(event.Kind))
				binary.BigEndian.PutUint32(key[2:6], uint32(event.CreatedAt))
				binary.BigEndian.PutUint32(key[6:10], serial)
				serial++

				err := db.lmdbEnv.Update(func(txn *lmdb.Txn) error {
					val, _ := easyjson.Marshal(event)
					return txn.Put(*group, key, val, 0)
				})
				if err != nil {
					return fmt.Errorf("failed to store event in database: %w", err)
				}
			case nostr.KindSimpleChatAction:
				// it's an action and we know it comes from a user allowed to perform it
				// because we checked for that on AcceptEvent, so just do it
				// TODO
			}
		}
	}

	return nil
}
