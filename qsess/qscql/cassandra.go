// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qscql is a Cassandra/Scylla back-end for qsess.
package qscql

// Expiration is handled very easily, by setting a time-to-live per session.
//
// DeleteByUserId can be handled via various strategies:
// (1) key the session table by (userid, sessid) - simplest and probably most
// efficient, but exposes userids to clients in (encrypted) cookies/tokens,
// (2) create a secondary index of sessions by userid - simple but Scylla
// doesn't have indexes (as of early 2017), or (3) make a separate table,
// used as an index - bad when indexed items can disappear via TTL expiration.
//
// This package implements #2 for Cassandra and #1 for Scylla.

import (
	"errors"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gocql/gocql"
)

// type cqlStore holds per-store information and implements SessBackEnd.
type cqlStore struct {
	db          *gocql.Session
	uidIndex    bool
	uidToClient bool
	qGet        string
	qInsert     string
	qDelete     string
	qDelByUID   string // delete all sessions for a user id
	qGetByUID   string // find all sessions for a user id, for manual deletion
}

// NewCqlStore creates a new session store, using a cassandra database.
//
// table is the name of a database table to hold session data
// (it will be created if it doesn't exist).
//
// cipherkeys are one or more 32-byte encryption keys, to be used with
// AES-GCM. For encryption, only the first key is used;
// for decryption all keys are tried (allowing key rotation).
//
// uidIndex and uidToClient control the implementation of DeleteByUserID.
// If you are not using DeleteByUserID, set them both to false.
// If you are using DeleteByUserID with Cassandra, set uidIndex to true,
// which creates a secondary index of sessions by userID.
// If you are using DeleteByUserID with Scylla, set uidToClient to true,
// which keys the session table by (userID, sessID), which requires that
// encrypted userIDs be stored in client cookies/tokens.
// There is never a reason to set them both to true.
//
// Additional configuration options can be set by manipulating fields in the
// returned qsess.Store.
func NewCqlStore(gs *gocql.Session, table string, uidIndex bool, uidToClient bool, cipherkeys ...[]byte) (*qsess.Store, error) {
	var key string

	cs := &cqlStore{
		db:          gs,
		uidIndex:    uidIndex,
		uidToClient: uidToClient,
		qInsert:     `INSERT INTO "` + table + `" (sessid, userid, data, maxage, minrefresh) VALUES(?, ?, ?, ?, ?) USING TTL ?`,
	}

	if uidToClient {
		key = "(userid, sessid)"
		cs.qGet = `SELECT data, userid, TTL(data), maxage, minrefresh FROM "` + table + `" WHERE userid = ? AND sessid = ?`
		cs.qDelete = `DELETE FROM "` + table + `" WHERE userid = ? AND sessid = ?`
		cs.qDelByUID = `DELETE FROM "` + table + `" WHERE userid = ?`

	} else {
		key = "(sessid)"
		cs.qGet = `SELECT data, userid, TTL(data), maxage, minrefresh FROM "` + table + `" WHERE sessid = ?`
		cs.qDelete = `DELETE FROM "` + table + `" WHERE sessid = ?`
		cs.qGetByUID = `SELECT sessid FROM "` + table + `" WHERE userid = ?`
	}

	err := gs.Query(`CREATE TABLE IF NOT EXISTS "` + table +
		`" ( sessid uuid, userid blob, data blob, maxage int, minrefresh int, PRIMARY KEY ` + key +
		` ) WITH gc_grace_seconds = 86400 AND compaction = { 'class':'LeveledCompactionStrategy'}`).Exec()
	if err != nil {
		return &qsess.Store{}, errors.New("NewCqlStore - CREATE TABLE failed - " + err.Error())
	}

	if uidIndex {
		err = gs.Query(`CREATE INDEX IF NOT EXISTS "` + table + `_uid_ndx" ON ` + table + ` (userid)`).Exec()
		if err != nil {
			return &qsess.Store{}, errors.New("NewCqlStore - CREATE INDEX failed - " + err.Error())
		}
	}

	st, err := qsess.NewStore(cs, uidToClient, cipherkeys...)
	if err != nil {
		return nil, errors.New("NewCqlStore - NewStore - " + err.Error())
	}

	return st, nil
}

func (c *cqlStore) Get(sessID []byte, userID []byte) ([]byte, []byte, int, int, int, error) {
	var data []byte
	var ttl, maxage, minrefresh int
	var err error
	if c.uidToClient {
		err = c.db.Query(c.qGet).Bind(userID, bytesToID(sessID)).Scan(&data, &userID, &ttl, &maxage, &minrefresh)
	} else {
		err = c.db.Query(c.qGet).Bind(bytesToID(sessID)).Scan(&data, &userID, &ttl, &maxage, &minrefresh)
	}
	return data, userID, ttl, maxage, minrefresh, err
}

func (c *cqlStore) Save(sessID *[]byte, data []byte, userID []byte, maxage int, minrefresh int) error {
	if *sessID == nil {
		// this is the first Save of a new session; generate a new key.
		*sessID = gocql.UUIDFromTime(time.Now()).Bytes()
	}
	return c.db.Query(c.qInsert).Bind(*sessID, userID, data, maxage, minrefresh, maxage).Exec()
}

func (c *cqlStore) Delete(sessID []byte, userID []byte) error {
	if c.uidToClient {
		return c.db.Query(c.qDelete).Bind(userID, bytesToID(sessID)).Exec()
	}
	return c.db.Query(c.qDelete).Bind(bytesToID(sessID)).Exec()
}

func (c *cqlStore) DeleteByUserID(userID []byte) error {
	var sessID []byte
	var err error

	if (!c.uidIndex) && (!c.uidToClient) {
		return errors.New("cqlStore.DeleteByUserID - require uidIndex or uidToClient")
	}

	if c.uidToClient {
		return c.db.Query(c.qDelByUID, userID).Exec()
	} else {
		// even if we have an index, we can't delete using a WHERE clause that
		// doesn't include the partition key, so do a SELECT and delete each
		// session explicitly. this is OK, because (we assume) DeleteByUserID
		// happens rarely, and the number of sessions per userid is very small.
		iter := c.db.Query(c.qGetByUID, userID).Iter()

		// try to delete all, in spite of errors (if any).
		// if errors, return the first one.
		err = nil
		for iter.Scan(&sessID) {
			curErr := c.db.Query(c.qDelete).Bind(bytesToID(sessID)).Exec()
			if curErr != nil && err == nil {
				err = curErr
			}
		}
		closeErr := iter.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
}

// serialize gocql.UUIDs, which we use as session ids (database keys).

func bytesToID(src []byte) gocql.UUID {
	// could check error return, but depend on qsess promise not to touch
	u, _ := gocql.UUIDFromBytes(src)
	return u
}
