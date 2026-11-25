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
// - An expiration index record exists without a corresponding session record,
//   or names an expiration time its session record no longer holds.
//   This is OK, because the pruner will delete it at that time.
//
// - A user id index record exists without a corresponding session record.
//   This is OK, because the next DeleteByUserID for that user id deletes it.
//   DeleteByUserID deletes the session record first, so that an interrupted
//   call leaves no live session out of the reach of the next.
//
// - A session record exists without a user id index entry.
//   If DeleteByUserID is called, that session would not be deleted.
//   Since this inconsistency is likey to be extremely rare AND the use of
//   DeleteByUserID is also rare, the likelihood of their intersection is
//   essentially zero. If we nevertheless wanted to completely eliminate that
//   possibilty, we could do it by re-arranging the sequence of operations,
//   as DeleteByUserID does, which leaves more of the user id index records
//   above, but it doesn't seem worth the trouble, for such an unlikely scenario.

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const DefaultPruneIntervalSecs = 2 * 60 // prune every 2 minutes

// gldbDB is the part of *leveldb.DB that a store uses, so that a test can make any one of its calls fail.
type gldbDB interface {
	Get(key []byte, ro *opt.ReadOptions) ([]byte, error)
	Put(key, value []byte, wo *opt.WriteOptions) error
	Delete(key []byte, wo *opt.WriteOptions) error
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
}

// type gldbStore holds per-store information and implements SessBackEnd

type gldbStore struct {
	db gldbDB

	prefixSize int // size of key prefixes used to distinguish record types

	sessPrefix []byte // key prefix for session table records
	expPrefix  []byte // key prefix for expiration index records
	uidPrefix  []byte // key prefix for user id index records

	sessKeySize int
	expKeySize  int

	// mu is held by every write to a session's records, from the read that the write depends on, if any, until
	// the write is done, so that no write acts on a record that has changed since it was read.
	mu sync.Mutex
}

// NewGldbStore creates a new session store, using a goleveldb database.
//
// prefix will be prepended to database keys, so that a session's data can
// coexist with other data within the database. If the goleveldb database
// will not be used for anything besides session storage, and there is only
// a single goleveldb session store, prefix can be empty. It is also the
// Store.Name, so that stores that share keys do not accept each other's
// values, and a store takes a value that names another store's session, as
// an unbound one that qsess.Store.AcceptUnbound accepts or one from an
// application's own Decrypt can, for one whose session is gone. No other
// store on the same database may use the same prefix: the two would share
// sessions but not the lock that keeps their changes to a session from
// undoing each other.
//
// cipherkeys are one or more 32-byte encryption keys, to be used with
// AES-GCM. For encryption, only the first key is used;
// for decryption all keys are tried (allowing key rotation).
//
// User ids may be at most 255 bytes long.
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
		return nil, gldbErr{"NewGldbStore - NewStore", err}
	}
	st.Name = string(prefix)
	st.PruneInterval = make(chan int)
	st.PruneKill = make(chan int)

	go gst.prune(DefaultPruneIntervalSecs, st.PruneInterval, st.PruneKill, errLog)

	return st, nil
}

func (gst *gldbStore) Get(sessID []byte, uidNOTUSED []byte) (data []byte, userID []byte, timeToLiveSecs int, maxAgeSecs int, minRefreshSecs int, err error) {
	if !gst.ownSessKey(sessID) {
		err = gldbErr{"gldbStore.Get - not this store's session", qsess.ErrNotFound}
		return
	}
	data, err = gst.db.Get(sessID, nil)
	if err != nil {
		err = gldbErr{"gldbStore.Get", notFound(err)}
		return
	}

	sessVal := gldbSessValue(data)
	if !sessVal.valid() {
		err = gldbErr{"gldbStore.Get - malformed session record", nil}
		return
	}
	ttl := sessVal.expiration() - time.Now().Unix()
	if ttl <= 0 {
		gst.deleteExpired(sessID)
		err = gldbErr{"gldbStore.Get - expired", qsess.ErrNotFound}
		return
	}

	return sessVal.data(), sessVal.userID(), int(ttl), int(sessVal.maxage()), int(sessVal.minrefresh()), nil
}

// deleteExpired reads the record again under mu, because a Save can refresh a session after Get has found it
// expired.
func (gst *gldbStore) deleteExpired(sessID []byte) {
	gst.mu.Lock()
	defer gst.mu.Unlock()

	data, err := gst.db.Get(sessID, nil)
	if err != nil {
		return
	}
	sessVal := gldbSessValue(data)
	if sessVal.valid() && sessVal.expiration() <= time.Now().Unix() {
		gst.deleteSession(sessID, sessVal)
	}
}

func (gst *gldbStore) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	gst.mu.Lock()
	defer gst.mu.Unlock()

	var sessKey gldbSessKey
	var firstSave bool
	var oldExpKey gldbExpKey

	if *sessID == nil {
		// this is the first Save of a new session; generate a unique key.
		sessKey = gst.newSessKey()
		firstSave = true
	} else {
		sessKey = *sessID
		if !gst.ownSessKey(sessKey) {
			return gldbErr{"gldbStore.Save - not this store's session", qsess.ErrNotFound}
		}
		// see if session exists; could be gone via expiration or DeleteByUserId
		oldData, err := gst.db.Get(sessKey, nil)
		if err != nil {
			return gldbErr{"gldbStore.Save - Get", notFound(err)}
		}
		oldSessVal := gldbSessValue(oldData)
		if !oldSessVal.valid() {
			return gldbErr{"gldbStore.Save - malformed session record", nil}
		}
		if oldSessVal.expiration() <= time.Now().Unix() {
			return gldbErr{"gldbStore.Save - expired", qsess.ErrNotFound}
		}
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

	expKey := gst.expKey(sessVal.expirationBytes(), sessKey)
	if err := gst.db.Put(expKey, []byte{}, nil); err != nil {
		return gldbErr{"gldbStore.Save - expiration index Put", err}
	}

	if err := gst.db.Put(sessKey, sessVal, nil); err != nil {
		return gldbErr{"gldbStore.Save - session Put", err}
	}

	if firstSave {
		if err := gst.db.Put(gst.uidKey(userID, sessKey), []byte{}, nil); err != nil {
			return gldbErr{"gldbStore.Save - userid index Put", err}
		}
		*sessID = sessKey
	} else if !bytes.Equal(oldExpKey, expKey) {
		// a Save within a second of the last one puts the same index record back, and one that a failed delete
		// leaves is the pruner's to delete, as the refresh has already taken effect
		gst.db.Delete(oldExpKey, nil)
	}

	return nil
}

func (gst *gldbStore) Delete(sessID []byte, uidNOTUSED []byte) error {
	if !gst.ownSessKey(sessID) {
		return gldbErr{"gldbStore.Delete - not this store's session", qsess.ErrNotFound}
	}
	gst.mu.Lock()
	defer gst.mu.Unlock()

	data, err := gst.db.Get(sessID, nil)
	if err != nil {
		return gldbErr{"gldbStore.Delete - Get", notFound(err)}
	}
	sessVal := gldbSessValue(data)
	if !sessVal.valid() {
		return gldbErr{"gldbStore.Delete - malformed session record", nil}
	}
	if err := gst.deleteSession(sessID, sessVal); err != nil {
		return gldbErr{"gldbStore.Delete", err}
	}
	return nil
}

// deleteSession deletes the session record sessVal, read from key sessID, with its index entries. It reports
// only a failure to delete the session record, because the pruner and DeleteByUserID delete index entries left.
func (gst *gldbStore) deleteSession(sessID []byte, sessVal gldbSessValue) error {
	// sequence: user id index entry, session record, expiration index entry
	// so we can't leak anything if we get interrupted.

	gst.db.Delete(gst.uidKey(sessVal.userID(), sessID), nil)

	if err := gst.db.Delete(sessID, nil); err != nil {
		return gldbErr{"gldbStore.deleteSession - session Delete", err}
	}

	gst.db.Delete(gst.expKey(sessVal.expirationBytes(), sessID), nil)

	return nil
}

// Consume implements qsess.Consumer. Holding mu across the read and the deletes is enough to
// redeem a session exactly once, and to keep a Save from bringing it back, because goleveldb holds
// the database open exclusively, no other store may share the prefix (see NewGldbStore) and none
// writes a session it did not issue (see ownSessKey), so this store is the only writer of its
// records; transactions are too expensive here (see the notes at the top of this file).
func (gst *gldbStore) Consume(sessID []byte, uidNOTUSED []byte) ([]byte, []byte, int, int, int, error) {
	if !gst.ownSessKey(sessID) {
		return []byte{}, []byte{}, 0, 0, 0, gldbErr{"gldbStore.Consume - not this store's session", qsess.ErrNotFound}
	}
	gst.mu.Lock()
	defer gst.mu.Unlock()

	record, err := gst.db.Get(sessID, nil)
	if err != nil {
		return []byte{}, []byte{}, 0, 0, 0, gldbErr{"gldbStore.Consume - Get", notFound(err)}
	}
	sessVal := gldbSessValue(record)
	if !sessVal.valid() {
		return []byte{}, []byte{}, 0, 0, 0, gldbErr{"gldbStore.Consume - malformed session record", nil}
	}
	if err := gst.deleteSession(sessID, sessVal); err != nil {
		return []byte{}, []byte{}, 0, 0, 0, gldbErr{"gldbStore.Consume", err}
	}

	// the session is gone either way, which is what the pruner would have done
	ttl := sessVal.expiration() - time.Now().Unix()
	if ttl <= 0 {
		return []byte{}, []byte{}, 0, 0, 0, gldbErr{"gldbStore.Consume - record has expired", qsess.ErrNotFound}
	}

	return sessVal.data(), sessVal.userID(), int(ttl), int(sessVal.maxage()), int(sessVal.minrefresh()), nil
}

// DeleteByUserID deletes every session that the user id index lists for userID, carrying on past a failure so as
// to revoke what it can, and reports the first failure that can leave a session valid.
func (gst *gldbStore) DeleteByUserID(userID []byte) error {
	prefix := gst.uidKeyPrefix(userID)
	iter := gst.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	var firstErr error
	for iter.Next() {
		uxkey := gldbUIDKey(iter.Key())
		// NOTE: nothing in an index key ends the user id, so the scan also finds longer user ids that begin
		// with this one, which the key's length tells apart. Like the pruner, it leaves alone a key that names
		// none of this store's session keys or holds a value, which is another store's or the application's.
		if len(uxkey) != len(prefix)+gst.sessKeySize || !gst.ownSessKey(uxkey.sessKey(gst.prefixSize)) ||
			len(iter.Value()) != 0 {
			continue
		}
		if err := gst.deleteUserSession(uxkey); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := iter.Error(); err != nil && firstErr == nil {
		firstErr = gldbErr{"gldbStore.DeleteByUserID - user id index scan", err}
	}
	return firstErr
}

// deleteUserSession deletes uxkey, a user id index record, with the session record it names and, where that record
// can be read and holds an expiration time, the session's expiration index record. It reports only a failure that
// can leave the session valid, because the next DeleteByUserID and the pruner delete the index records left.
func (gst *gldbStore) deleteUserSession(uxkey gldbUIDKey) error {
	gst.mu.Lock()
	defer gst.mu.Unlock()

	skey := uxkey.sessKey(gst.prefixSize)
	data, getErr := gst.db.Get(skey, nil)

	// sequence: the session record first, so that if we get interrupted the session is gone and the next
	// DeleteByUserID, or the pruner, finds what is left. One that cannot be read goes too, as revoking it needs
	// only its key.
	if !errors.Is(getErr, leveldb.ErrNotFound) {
		if err := gst.db.Delete(skey, nil); err != nil {
			return gldbErr{"gldbStore.DeleteByUserID - session Delete", err}
		}
	}
	gst.db.Delete(uxkey, nil)
	if getErr == nil && len(data) >= bytesPerInt64 {
		gst.db.Delete(gst.expKey(data[:bytesPerInt64], skey), nil)
	}
	return nil
}

// ownSessKey reports whether key is one of this store's session keys, the only ones Get, Save, Delete and Consume
// accept: another store's session is guarded by that store's lock, and a value that names one should get no
// further than it would were it bound to that store.
func (gst *gldbStore) ownSessKey(key []byte) bool {
	return len(key) == gst.sessKeySize && bytes.HasPrefix(key, gst.sessPrefix)
}
