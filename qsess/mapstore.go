// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// simple back-end for qsess, using an in-memory map.
//
// expiration - mapSess.expireTime tracks expiration for each session.
// DeleteByUserId - maintain a separate userid index in memory.

package qsess

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"sync"
	"time"
)

type mapSess struct {
	data           []byte
	userID         string // converted from byte slices, for use as map keys
	expireTime     int64
	maxAgeSecs     int
	minRefreshSecs int
}

// type mapStore holds per-store information and implements SessBackEnd.
type mapStore struct {
	sync.RWMutex

	sess   map[uint32]mapSess
	nextID uint32

	// index of session ids by userid, for DeleteByUserId
	uindex map[string]map[uint32]struct{}
}

const (
	mapPruneIntervalSecs = 2 * 60
	mapPruneBatch        = 1024
)

func (m *mapStore) uindexAdd(userID string, sessID uint32) {
	_, ok := m.uindex[userID]
	if !ok {
		m.uindex[userID] = make(map[uint32]struct{}, 1)
	}
	m.uindex[userID][sessID] = struct{}{}
}

// uindexRemove also removes userID once it has no sessions left, so that the index does not keep
// an entry for every user id it has seen.
func (m *mapStore) uindexRemove(userID string, sessID uint32) {
	delete(m.uindex[userID], sessID)
	if len(m.uindex[userID]) == 0 {
		delete(m.uindex, userID)
	}
}

// NewMapStore creates a new session store, using a simple, in-memory map,
// with no persistence.
//
// cipherkeys are one or more 32-byte encryption keys, to be used with
// AES-GCM. For encryption, only the first key is used;
// for decryption all keys are tried (allowing key rotation).
//
// Store.Name is random: nothing in a map store outlives the process, and map
// stores that share keys never accept each other's values.
//
// NewMapStore creates a goroutine, to delete expired sessions every two
// minutes, which you can control using two channels in the Store:
// PruneInterval and PruneKill. It keeps the store in memory until it receives
// on PruneKill, so a program that discards map stores, such as a test, must
// send on PruneKill for each one.
//
// Additional configuration options can be set by manipulating fields in the
// returned qsess.Store.
func NewMapStore(cipherkeys ...[]byte) (*Store, error) {
	ms := &mapStore{
		sync.RWMutex{},
		make(map[uint32]mapSess),
		1000,
		make(map[string]map[uint32]struct{}),
	}

	st, err := NewStore(ms, false, cipherkeys...)
	if err != nil {
		return nil, qsErr{"NewMapStore - NewStore", err}
	}

	name := make([]byte, 8)
	if _, err := rand.Read(name); err != nil {
		return nil, qsErr{"NewMapStore - could not create Name", err}
	}
	st.Name = hex.EncodeToString(name)

	st.PruneInterval = make(chan int)
	st.PruneKill = make(chan int)
	go ms.prune(mapPruneIntervalSecs, st.PruneInterval, st.PruneKill)

	return st, nil
}

func (m *mapStore) Get(sessIDbytes []byte, uidNOTUSED []byte) ([]byte, []byte, int, int, int, error) {
	sessID := bytesToID(sessIDbytes)

	m.RLock()
	s, ok := m.sess[sessID]
	m.RUnlock()

	if !ok {
		return []byte{}, []byte{}, 0, 0, 0, qsErr{"mapStore.Get - id not found", ErrNotFound}
	}
	ttl := s.expireTime - time.Now().Unix()
	if ttl <= 0 {
		m.deleteExpired(sessID)
		return []byte{}, []byte{}, 0, 0, 0, qsErr{"mapStore.Get - expired", ErrNotFound}
	}

	return s.data, []byte(s.userID), int(ttl), s.maxAgeSecs, s.minRefreshSecs, nil
}

// deleteExpired checks expiration again under the write lock, because a Save can refresh a
// session after a reader has found it expired.
func (m *mapStore) deleteExpired(sessID uint32) {
	m.Lock()
	defer m.Unlock()

	s, ok := m.sess[sessID]
	if ok && s.expireTime <= time.Now().Unix() {
		m.uindexRemove(s.userID, sessID)
		delete(m.sess, sessID)
	}
}

// prune deletes expired sessions every waitSecs seconds, until it receives on pruneKill; a value
// received on pruneInterval replaces waitSecs. It lets go of the lock every mapPruneBatch sessions,
// because once a writer is waiting even the read lock holds up every request for the rest of a pass.
func (m *mapStore) prune(waitSecs int, pruneInterval <-chan int, pruneKill <-chan int) {
	for {
		select {
		case waitSecs = <-pruneInterval:
		case <-pruneKill:
			return
		case <-time.After(time.Duration(waitSecs) * time.Second):
		}

		now := time.Now().Unix()
		visited := 0
		m.Lock()
		for sessID, s := range m.sess {
			if s.expireTime <= now {
				m.uindexRemove(s.userID, sessID)
				delete(m.sess, sessID)
			}
			visited++
			if visited%mapPruneBatch == 0 {
				m.Unlock()
				m.Lock()
			}
		}
		m.Unlock()
	}
}

// Consume implements qsess.Consumer. It holds the write lock across the read and the delete,
// so that of several concurrent calls for one session id exactly one finds it.
func (m *mapStore) Consume(sessIDbytes []byte, uidNOTUSED []byte) ([]byte, []byte, int, int, int, error) {
	m.Lock()
	defer m.Unlock()

	sessID := bytesToID(sessIDbytes)
	s, ok := m.sess[sessID]
	if !ok {
		return []byte{}, []byte{}, 0, 0, 0, qsErr{"mapStore.Consume - id not found", ErrNotFound}
	}
	m.uindexRemove(s.userID, sessID)
	delete(m.sess, sessID)

	// the session is gone either way, which is what expiration would have done
	ttl := s.expireTime - time.Now().Unix()
	if ttl <= 0 {
		return []byte{}, []byte{}, 0, 0, 0, qsErr{"mapStore.Consume - record has expired", ErrNotFound}
	}

	return s.data, []byte(s.userID), int(ttl), s.maxAgeSecs, s.minRefreshSecs, nil
}

func (m *mapStore) Save(sessIDbytes *[]byte, data []byte, userIDbytes []byte, maxAgeSecs int, minRefreshSecs int) error {
	m.Lock()
	defer m.Unlock()

	var sessID uint32
	if *sessIDbytes == nil {
		// this is the first Save of a new session; generate a new key.
		sessID = m.nextID
		m.nextID++
		*sessIDbytes = idToBytes(sessID)
		m.sess[sessID] = mapSess{}
	} else {
		sessID = bytesToID(*sessIDbytes)
		// see if session exists; could be gone via expiration or DeleteByUserId
		s, ok := m.sess[sessID]
		if !ok {
			return qsErr{"mapStore.Save - id not found", ErrNotFound}
		}
		if s.expireTime <= time.Now().Unix() {
			return qsErr{"mapStore.Save - expired", ErrNotFound}
		}
	}
	userID := string(userIDbytes)
	m.sess[sessID] = mapSess{
		data,
		userID,
		time.Now().Add(time.Duration(maxAgeSecs) * time.Second).Unix(),
		maxAgeSecs,
		minRefreshSecs,
	}
	m.uindexAdd(userID, sessID)
	return nil
}

func (m *mapStore) Delete(sessIDbytes []byte, uidNOTUSED []byte) error {
	m.Lock()
	defer m.Unlock()

	sessID := bytesToID(sessIDbytes)
	if s, ok := m.sess[sessID]; ok {
		m.uindexRemove(s.userID, sessID)
		delete(m.sess, sessID)
	}
	return nil
}

func (m *mapStore) DeleteByUserID(userIDbytes []byte) error {
	m.Lock()
	defer m.Unlock()

	userID := string(userIDbytes)
	for sessID := range m.uindex[userID] {
		delete(m.sess, sessID)
	}
	delete(m.uindex, userID)
	return nil
}

// serialize uint32, which we use to store a session id (database key).

func idToBytes(id uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, id)
	return b
}

func bytesToID(b []byte) uint32 {
	// could check len(b), but depend on qsess promise not to touch
	return binary.LittleEndian.Uint32(b)
}
