// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsldb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

// prune periodically deletes expired sessions from the session store.
// It uses an index, to locate expired sessions efficiently.
//
// prune runs in a goroutine, started by NewGldbStore.
// It runs until it receives something on its pruneKill channel.
// You can change its wait interval by sending a number of seconds
// to its pruneInterval channel.
func (gst *gldbStore) prune(waitSecs int, pruneInterval <-chan int, pruneKill <-chan int, log io.Writer) {
	for {
		select {
		case waitSecs = <-pruneInterval:
		case <-pruneKill:
			return
		case <-time.After(time.Duration(waitSecs) * time.Second):
		}

		now := time.Now().Unix()
		foreign := 0
		malformed := 0
		failed := 0
		var firstForeign, firstMalformed, firstFailed []byte
		var failedErr error
		iter := gst.db.NewIterator(nil, nil)
		// Giving Seek the key prefix brings us to the first record of the expiration index.
		for ok := iter.Seek(gst.expPrefix); ok; ok = iter.Next() {
			eKey := gldbExpKey(iter.Key())
			if !bytes.HasPrefix(eKey, gst.expPrefix) {
				// we're past the last session expiration index record
				break
			}
			if len(eKey) != gst.expKeySize || !gst.ownSessKey(eKey.sessKey(gst.prefixSize)) ||
				len(iter.Value()) != 0 {
				// Not one of this store's index records, which name one of its session keys and hold no value.
				// Another store whose prefix extends our expiration prefix sorts into this range, as do an
				// application key and an index record written for another store's session, so leave it alone
				// rather than delete data that cannot be proved to be ours, and log it for the operator.
				if foreign == 0 {
					firstForeign = bytes.Clone(eKey)
				}
				foreign++
				continue
			}
			if eKey.expiration(gst.prefixSize) >= now {
				// NOTE: keys hold the expiration time little-endian, so the index is not in
				// expiration order and expired records can follow this one.
				continue
			}
			isMalformed, err := gst.pruneSession(eKey, now)
			if isMalformed {
				if malformed == 0 {
					firstMalformed = bytes.Clone(eKey)
				}
				malformed++
			}
			if err != nil {
				if failed == 0 {
					firstFailed = bytes.Clone(eKey)
					failedErr = err
				}
				failed++
			}
		}
		iter.Release()

		if log != nil && foreign > 0 {
			fmt.Fprintf(log, "qsldb.prune - %d keys in the expiration index range are not this store's index "+
				"records, the first %x\n", foreign, firstForeign)
		}
		if log != nil && malformed > 0 {
			fmt.Fprintf(log, "qsldb.prune - %d expiration index records name a malformed session record, "+
				"the first %x\n", malformed, firstMalformed)
		}
		if log != nil && failed > 0 {
			fmt.Fprintf(log, "qsldb.prune - %d expiration index records name a session record that could not be "+
				"read or deleted, the first %x - %v\n", failed, firstFailed, failedErr)
		}
	}
}

// pruneSession deletes eKey, an expired expiration index record, and the session record it names if that has
// expired too, holding mu so that a Save cannot refresh the session in between. It reports malformed, deleting
// nothing, for a session record too short for its fields, which is not provably this store's either, and reports
// the error, keeping eKey for a later pass, for one that it cannot read or delete.
func (gst *gldbStore) pruneSession(eKey gldbExpKey, now int64) (malformed bool, err error) {
	gst.mu.Lock()
	defer gst.mu.Unlock()

	sessKey := eKey.sessKey(gst.prefixSize)
	data, err := gst.db.Get(sessKey, nil)
	if err == nil {
		sessData := gldbSessValue(data)
		if !sessData.valid() {
			return true, nil
		}
		if sessData.expiration() < now {
			err = gst.deleteSession(sessKey, sessData)
		}
	} else {
		err = gldbErr{"gldbStore.pruneSession - Get", err}
	}
	if err != nil && !errors.Is(err, leveldb.ErrNotFound) {
		// eKey may be the last index record to name the session record
		return false, err
	}
	// no session needs eKey now, so a failed delete only leaves it for the next pass
	gst.db.Delete(eKey, nil)
	return false, nil
}
