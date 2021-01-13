// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qsldb is a goleveldb back-end for qsess.
package qsldb

// Indexes are maintained for session expiration and DeleteByUserID.
//
// Ideally, we would use transactions to guarantee indices stay consistent
// with the session table, but goleveldb transactions are prohibitively
// expensive. In practice, we have not seen a database error in the middle of
// a sequence that should be atomic (but, of course, externally-induced
// program termination could produce such an inconsistency).
//
// With careful sequencing of database operations, we reduce the possible
// inconsistencies to only the following:
//
// - An expiration index record exists without a corresponding session record.
//   This is OK, because the pruner will delete it at its expiration time.
//
// - A session record exists without a user id index entry.
//   If DeleteByUserID is called, that session would not be deleted.
//   Since this inconsistency is likey to be extremely rare AND the use of
//   DeleteByUserID is also rare, the likelihood of their intersection is
//   essentially zero. If we nevertheless wanted to completely eliminate that
//   possibilty, we could do it by re-arranging the sequence of operations,
//   which would introduce the possibility of leaking user id index records,
//   which could be cleaned up by adding work to the pruner,
//   but it doesn't seem worth the trouble, for such an unlikely scenario.

import (
	"io"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const DefaultPruneIntervalSecs = 2 * 60 // prune every 2 minutes

// type gldbStore holds per-store information and implements SessBackEnd

type gldbStore struct {
	db *leveldb.DB

	prefixSize int // size of key prefixes used to distinguish record types

	sessPrefix []byte // key prefix for session table records
	expPrefix  []byte // key prefix for expiration index records
	uidPrefix  []byte // key prefix for user id index records

	sessKeySize int
	expKeySize  int
}

// NewGldbStore creates a new session store, using a goleveldb database.
//
// prefix will be prepended to database keys, so that a session's data can
// coexist with other data within the database. If the goleveldb database
// will not be used for anything besides session storage, and there is only
// a single goleveldb session store, prefix can be empty.
//
// cipherkeys are one or more 32-byte encryption keys, to be used with
// AES-GCM. For encryption, only the first key is used;
// for decryption all keys are tried (allowing key rotation).
//
// Additional configuration options can be set by manipulating fields in the
// returned qsess.Store.
//
// NewGldbStore creates a goroutine, to prune expired sessions from the
// database, using an index by expiration time.
// You can generally ignore the pruner goroutine, but, if necessary, you can
// control it using two channels in the Store: PruneInterval and PruneKill.
//
// errLog, if non-nil, enables the pruner goroutine to log unstructured
// error messages. No routine, "info" level messages are generated, only true
// errors, which should be acted on.
func NewGldbStore(db *leveldb.DB, prefix []byte, errLog io.Writer, cipherkeys ...[]byte) (*qsess.Store, error) {
	gst := &gldbStore{
		db:         db,
		prefixSize: len(prefix) + 1,
		sessPrefix: bscat(prefix, []byte{1}),
		expPrefix:  bscat(prefix, []byte{2}),
		uidPrefix:  bscat(prefix, []byte{3}),
	}

	gst.sessKeySize = sessKeySize(gst.prefixSize)
	gst.expKeySize = expKeySize(gst.prefixSize)

	st, err := qsess.NewStore(gst, false, cipherkeys...)
	if err != nil {
		return nil, gldbErr{"NewGldbStore - NewStore - ", err}
	}
	st.PruneInterval = make(chan int)
	st.PruneKill = make(chan int)

	go gst.prune(DefaultPruneIntervalSecs, st.PruneInterval, st.PruneKill, errLog)

	return st, nil
}

func (gst *gldbStore) Get(sessID []byte, uidNOTUSED []byte) (data []byte, userID []byte, timeToLiveSecs int, maxAgeSecs int, minRefreshSecs int, err error) {
	data, err = gst.db.Get(sessID, nil)
	if err != nil {
		err = gldbErr{"gldbStore.Get", err}
		return
	}

	if len(data) < sessValueFixedPartSize {
		err = gldbErr{"gldbStore.Get - malformed session record", nil}
		return
	}
	sessVal := gldbSessValue(data)
	ttl := sessVal.expiration() - time.Now().Unix()
	if ttl <= 0 {
		gst.Delete(sessID, nil)
		err = gldbErr{"gldbStore.Get - expired", nil}
		return
	}

	return sessVal.data(), sessVal.userID(), int(ttl), int(sessVal.maxage()), int(sessVal.minrefresh()), nil
}

func (gst *gldbStore) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	var sessKey gldbSessKey
	var firstSave bool
	var oldExpKey gldbExpKey

	if *sessID == nil {
		// this is the first Save of a new session; generate a unique key.
		sessKey = gst.newSessKey()
		*sessID = sessKey
		firstSave = true
	} else {
		sessKey = *sessID
		// see if session exists; could be gone via expiration or DeleteByUserId
		oldData, err := gst.db.Get(sessKey, nil)
		if err != nil {
			return gldbErr{"gldbStore.Save - session not found", nil}
		}
		if len(oldData) < sessValueFixedPartSize {
			return gldbErr{"gldbStore.Save - malformed session record", nil}
		}
		oldSessVal := gldbSessValue(oldData)
		oldExpKey = gst.expKey(oldSessVal.expirationBytes(), sessKey)
	}

	sessVal, err := newSessValue(len(userID), len(data))
	if err != nil {
		return gldbErr{"gldbStore.Save - newSessValue", err}
	}
	itob(sessVal.expirationBytes(), time.Now().Add(time.Duration(maxAgeSecs)*time.Second).Unix())
	itob(sessVal.maxageBytes(), int64(maxAgeSecs))
	itob(sessVal.minrefreshBytes(), int64(minRefreshSecs))
	copy(sessVal.userID(), userID)
	copy(sessVal.data(), data)

	// sequence: expiration index entry, session record, user id index entry
	// so we can't leak anything if we get interrupted.

	if err := gst.db.Put(gst.expKey(sessVal.expirationBytes(), sessKey), []byte{}, nil); err != nil {
		return gldbErr{"gldbStore.Save - expiration index Put", err}
	}

	if err := gst.db.Put(sessKey, sessVal, nil); err != nil {
		return gldbErr{"gldbStore.Save - session Put", err}
	}

	if firstSave {
		if err := gst.db.Put(gst.uidKey(userID, sessKey), []byte{}, nil); err != nil {
			return gldbErr{"gldbStore.Save - userid index Put", err}
		}
	} else {
		if err := gst.db.Delete(oldExpKey, nil); err != nil {
			return gldbErr{"gldbStore.Save - old expiration index Delete", err}
		}
	}

	return nil
}

func (gst *gldbStore) Delete(sessID []byte, uidNOTUSED []byte) error {
	data, err := gst.db.Get(sessID, nil)
	if err != nil {
		return gldbErr{"gldbStore.Delete - Get", err}
	}
	if len(data) < sessValueFixedPartSize {
		return gldbErr{"gldbStore.Delete - malformed session record", nil}
	}
	sessVal := gldbSessValue(data)

	// sequence: user id index entry, session record, expiration index entry
	// so we can't leak anything if we get interrupted.

	gst.db.Delete(gst.uidKey(sessVal.userID(), sessID), nil)

	if err := gst.db.Delete(sessID, nil); err != nil {
		return gldbErr{"gldbStore.Delete - session Delete", err}
	}

	if err := gst.db.Delete(gst.expKey(sessVal.expirationBytes(), sessID), nil); err != nil {
		return gldbErr{"gldbStore.Delete - exp index Delete", err}
	}

	return nil
}

func (gst *gldbStore) DeleteByUserID(userID []byte) error {
	// since sessions can disappear via expiration, it's not necessarily
	// an error for any (or all) of these deletes to fail,
	// so just plow ahead, attempting everything, then blithely return nil

	// find sessions using index of sessions by userid
	iter := gst.db.NewIterator(util.BytesPrefix(gst.uidKeyPrefix(userID)), nil)
	for iter.Next() {
		uxkey := gldbUIDKey(iter.Key())
		skey := uxkey.sessKey(gst.prefixSize)
		expir, expErr := gst.findExpiration(skey)
		gst.db.Delete(skey, nil)
		gst.db.Delete(uxkey, nil)
		if expErr == nil {
			gst.db.Delete(gst.expKey(expir, skey), nil)
		}
	}
	iter.Release()
	return nil
}

// given a session key, read its session record and return its expiration time
func (gst *gldbStore) findExpiration(sessKey gldbSessKey) ([]byte, error) {
	data, err := gst.db.Get(sessKey, nil)
	if err != nil {
		return []byte{}, gldbErr{"gldbStore.findExpiration - Get", err}
	}
	if len(data) < sessValueFixedPartSize {
		return []byte{}, gldbErr{"gldbStore.findExpiration - malformed session record", nil}
	}
	sessVal := gldbSessValue(data)
	return sessVal.expirationBytes(), nil
}
