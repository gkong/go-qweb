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
		// Giving Seek the key prefix brings us to the first record of the expiration index.
		for ok := iter.Seek(gst.expPrefix); ok; ok = iter.Next() {
			eKey := gldbExpKey(iter.Key())
			if !bytes.HasPrefix(eKey, gst.expPrefix) {
				// we're past the last session expiration index record
				break
			}
			if len(eKey) != gst.expKeySize {
				// Not one of this store's index records. Another store whose prefix extends
				// ours sorts into this range, as does an application key, and nothing in the
				// key says whose it is, so leave it alone rather than delete data that cannot
				// be proved to be ours. The collision is a setup error, hence the log line.
				if log != nil {
					fmt.Fprintf(log, "qsldb.prune - key in expiration index range is not this store's - %x\n", eKey)
				}
				continue
			}
			if eKey.expiration(gst.prefixSize) >= now {
				// NOTE: keys hold the expiration time little-endian, so the index is not in
				// expiration order and expired records can follow this one.
				continue
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
