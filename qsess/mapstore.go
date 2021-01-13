// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// simple back-end for qsess, using an in-memory map.
//
// expiration - mapSess.expireTime tracks expiration for each session.
// DeleteByUserId - maintain a separate userid index in memory.

package qsess

import (
	"encoding/binary"
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

func (m *mapStore) uindexAdd(userID string, sessID uint32) {
	_, ok := m.uindex[userID]
	if !ok {
		m.uindex[userID] = make(map[uint32]struct{}, 1)
	}
	m.uindex[userID][sessID] = struct{}{}
}

// NewMapStore creates a new session store, using a simple, in-memory map,
// with no persistence.
//
// cipherkeys are one or more 32-byte encryption keys, to be used with
// AES-GCM. For encryption, only the first key is used;
// for decryption all keys are tried (allowing key rotation).
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
		return nil, qsErr{"NewMapStore - NewStore - ", err}
	}

	return st, nil
}

func (m *mapStore) Get(sessIDbytes []byte, uidNOTUSED []byte) ([]byte, []byte, int, int, int, error) {
	m.RLock()
	defer m.RUnlock()

	sessID := bytesToID(sessIDbytes)
	s, ok := m.sess[sessID]
	if !ok {
		return []byte{}, []byte{}, 0, 0, 0, qsErr{"mapStore.Get - id not found", nil}
	}
	ttl := s.expireTime - time.Now().Unix()
	if ttl <= 0 {
		delete(m.uindex[s.userID], sessID)
		delete(m.sess, sessID)
		return []byte{}, []byte{}, 0, 0, 0, qsErr{"mapStore.Get - key not found", nil}
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
		if _, ok := m.sess[sessID]; !ok {
			return qsErr{"mapStore.Save - id not found", nil}
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
	delete(m.uindex[m.sess[sessID].userID], sessID)
	delete(m.sess, sessID)
	return nil
}

func (m *mapStore) DeleteByUserID(userIDbytes []byte) error {
	m.Lock()
	defer m.Unlock()

	userID := string(userIDbytes)
	for sessID := range m.uindex[userID] {
		delete(m.sess, sessID)
		delete(m.uindex[userID], sessID)
	}
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
