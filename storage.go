package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/mailru/easyjson"
	"github.com/nbd-wtf/go-nostr"
)

const ROOT_CHANNEL_ID = "_"

var serial uint32

type lmdbchatbackend struct {
	lmdbPath string
	lmdbEnv  *lmdb.Env

	// events are indexed by their created_at timestamp only, then a serial integer for order consistency
	channels map[string]*lmdb.DBI // each "channel" is a channel like on discord, a room with an id, the "" channel is the default
	mutex    sync.Mutex
}

func (db *lmdbchatbackend) Init() error {
	// initialize lmdb
	env, err := lmdb.NewEnv()
	if err != nil {
		return err
	}

	env.SetMaxDBs(21)
	env.SetMaxReaders(500)
	env.SetMapSize(1 << 38) // ~273GB

	err = env.Open(db.lmdbPath, lmdb.NoTLS, 0644)
	if err != nil {
		return err
	}
	db.lmdbEnv = env

	// create channels map of dbis
	db.channels = make(map[string]*lmdb.DBI)

	return nil
}

func (db *lmdbchatbackend) getChannel(id string) (*lmdb.DBI, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if channel, ok := db.channels[id]; ok {
		return channel, nil
	}

	// not opened yet, so open and store it
	var channel *lmdb.DBI
	if err := db.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		if dbi, err := txn.OpenDBI(id, lmdb.Create); err != nil {
			return err
		} else {
			channel = &dbi
			db.channels[id] = &dbi
			return nil
		}
	}); err != nil {
		return nil, err
	}

	return channel, nil
}

func (db *lmdbchatbackend) QueryEvents(ctx context.Context, filter *nostr.Filter) (ch chan *nostr.Event, err error) {
	ch = make(chan *nostr.Event)

	// we only host kind 9
	if len(filter.Kinds) > 1 || (len(filter.Kinds) == 1 && filter.Kinds[0] != 9) {
		return ch, nil
	}

	// we only support querying by the "c" tag for now
	if len(filter.Tags) > 1 {
		return ch, nil
	} else if len(filter.Tags) == 1 {
		_, ok := filter.Tags["c"]
		if !ok {
			// there is a query for some other tag except "c"
			return ch, nil
		}
	}

	channelIds, _ := filter.Tags["c"]
	if len(channelIds) == 0 {
		channelIds = []string{ROOT_CHANNEL_ID}
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
	wg.Add(len(channelIds))
	for _, channelId := range channelIds {
		go func(channelId string) {
			defer wg.Done()

			if channel, err := db.getChannel(channelId); err != nil {
				log.Error().Err(err).Str("channel", channelId).Msg("failed to get channel")
				return
			} else {
				db.lmdbEnv.View(func(txn *lmdb.Txn) error {
					txn.RawRead = true

					cursor, err := txn.OpenCursor(*channel)
					if err != nil {
						log.Error().Err(err).Str("channel", channelId).Msg("failed to open cursor")
						return err
					}
					defer cursor.Close()

					initial := make([]byte, 4)
					binary.BigEndian.PutUint32(initial, until)
					nextKey := initial

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
		}(channelId)
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
	channelTag := event.Tags.GetFirst([]string{"c", ""})
	channelId := ROOT_CHANNEL_ID
	if channelTag != nil {
		channelId = (*channelTag)[1]
	}

	if channel, err := db.getChannel(channelId); err != nil {
		return fmt.Errorf("failed to open channel db: %w", err)
	} else {
		key := make([]byte, 8)
		binary.BigEndian.PutUint32(key, uint32(event.CreatedAt))
		binary.BigEndian.PutUint32(key[4:], serial)
		serial++

		err := db.lmdbEnv.Update(func(txn *lmdb.Txn) error {
			val, _ := easyjson.Marshal(event)
			return txn.Put(*channel, key, val, 0)
		})
		if err != nil {
			return fmt.Errorf("failed to store event in database: %w", err)
		}

		return nil
	}
}
