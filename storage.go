package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/mailru/easyjson"
	"github.com/nbd-wtf/go-nostr"
)

var serial uint32

type lmdbchatbackend struct {
	lmdbPath string
	lmdbEnv  *lmdb.Env

	// events are indexed by their created_at timestamp only, then a serial integer for order consistency
	channels map[string]*lmdb.DBI // each "channel" is a channel like on discord, a room with an id, the "" channel is the default
	mutex    sync.Mutex
}

func (db lmdbchatbackend) Init() error {
	// initialize lmdb
	env, err := lmdb.NewEnv()
	if err != nil {
		return err
	}

	env.SetMaxDBs(21)

	err = env.Open(db.lmdbPath, 0, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (db lmdbchatbackend) getChannel(id string) (*lmdb.DBI, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if channel, ok := db.channels[id]; ok {
		return channel, nil
	}

	// not opened yet, so open and store it
	var channel *lmdb.DBI
	if err := db.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		if dbi, err := txn.OpenDBI(id, 0); err != nil {
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

func (db lmdbchatbackend) QueryEvents(filter *nostr.Filter) (events []nostr.Event, err error) {
	channelIds, _ := filter.Tags["c"]
	if len(channelIds) == 0 {
		channelIds = []string{""}
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
				return
			} else {
				db.lmdbEnv.View(func(txn *lmdb.Txn) error {
					txn.RawRead = true

					cursor, err := txn.OpenCursor(*channel)
					if err != nil {
						return err
					}
					defer cursor.Close()

					initial := make([]byte, 8)
					binary.BigEndian.PutUint32(initial, until)

					_, _, err = cursor.Get(initial, nil, lmdb.SetRange)
					if err != nil {
						return err
					}

					i := 0
					for {
						k, v, err := cursor.Get(nil, nil, lmdb.PrevNoDup)
						if err != nil {
							break
						}

						if ts := binary.BigEndian.Uint32(k[0:4]); ts < since {
							break
						}

						var evt nostr.Event
						if err := json.Unmarshal(v, &evt); err != nil {
							continue
						}

						events = append(events, evt)

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

	wg.Wait()
	return events, nil
}

func (db lmdbchatbackend) DeleteEvent(id string, pubkey string) error {
	return fmt.Errorf("delete functionality not implemented")
}

func (db lmdbchatbackend) SaveEvent(event *nostr.Event) error {
	channelTag := event.Tags.GetFirst([]string{"c", ""})
	channelId := ""
	if channelTag != nil {
		channelId = (*channelTag)[1]
	}

	if channel, err := db.getChannel(channelId); err != nil {
		return fmt.Errorf("failed to open channel db on save: %w", err)
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
