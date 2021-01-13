// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsldb

import (
	"bytes"
	"fmt"
	"io"
	"time"
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
		iter := gst.db.NewIterator(nil, nil)
		// Giving Seek the key prefix brings us to the first record of the
		// expiration index, which is ordered by expiration time (ascending).
		for ok := iter.Seek(gst.expPrefix); ok; ok = iter.Next() {
			eKey := gldbExpKey(iter.Key())
			if bytes.Compare(eKey[:gst.prefixSize], gst.expPrefix) != 0 {
				// we're past the last session expiration index record
				break
			}
			if len(eKey) != gst.expKeySize {
				if log != nil {
					fmt.Fprintf(log, "qsess.Store.prune - pruning malformed expKey - %x", eKey)
				}
				gst.db.Delete(eKey, nil)
				continue
			}
			if eKey.expiration(gst.prefixSize) >= now {
				// we're past the records of interest
				break
			}
			// Current eKey's expiration time is in the past.
			// Read the session record and verify it's really expired,
			// then delete the session record and index record.
			// Ignore deletion failures, since other concurrent code can
			// delete these records.
			sessKey := eKey.sessKey(gst.prefixSize)
			data, err := gst.db.Get(sessKey, nil)
			if err == nil {
				sessData := gldbSessValue(data)
				if sessData.expiration() < now {
					gst.db.Delete(gst.uidKey(sessData.userID(), sessKey), nil)
					gst.db.Delete(sessKey, nil)
				}
			}
			gst.db.Delete(eKey, nil)
		}
		iter.Release()
	}
}
