// Copyright 2021 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// XXX - test that the indices are make prune() and DeleteByUserID() perform well when there are lots ofsessions

// XXX - test that prune() keeps old expired sessions from wasting space in the database

// Package qspgx is a back-end for qsess which uses PostgreSQL, accessed via the pgx package.
package qspgx

import (
	"context"
	"encoding/binary"
	"io"
	"strconv"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/jackc/pgx/v4/pgxpool"
)

const DefaultPruneIntervalSecs = 2 * 60 // prune every 2 minutes

var noctx = context.Background()

// type pgxStore holds per-store information and conforms to the SessBackEnd interface.
type pgxStore struct {
	db    *pgxpool.Pool
	table string
}

// NewPgxStore creates a new session store, using a PostgreSQL database accessed via pgxpool.
//
// table is the name of a database table to hold session data (it will be created if it doesn't exist).
//
// cipherkeys are one or more 32-byte encryption keys, to be used with AES-GCM.
// For encryption, only the first key is used; for decryption all keys are tried (allowing key rotation).
//
// Additional configuration options can be set by manipulating fields in the returned qsess.Store.
func NewPgxStore(pdb *pgxpool.Pool, tableName string, errLog io.Writer, cipherkeys ...[]byte) (*qsess.Store, error) {
	ps := &pgxStore{db: pdb, table: tableName}

	st, err := qsess.NewStore(ps, false, cipherkeys...)
	if err != nil {
		return nil, pgxErr{"NewPgxStore - NewStore - ", err}
	}

	st.PruneInterval = make(chan int)
	st.PruneKill = make(chan int)

	go ps.prune(DefaultPruneIntervalSecs, st.PruneInterval, st.PruneKill, errLog)

	_, err = pdb.Exec(noctx,
		`CREATE TABLE IF NOT EXISTS `+tableName+` (
			id SERIAL PRIMARY KEY,
			data BYTEA,
			userid BYTEA,
			expires TIMESTAMP NOT NULL,
			maxage INTEGER,
			minrefresh INTEGER
		 )`)
	if err != nil {
		return st, pgxErr{"NewPgxStore - CREATE TABLE failed - ", err}
	}

	_, err = pdb.Exec(noctx, `CREATE INDEX IF NOT EXISTS `+tableName+`_userid ON `+tableName+` (userid)`)
	if err != nil {
		return st, pgxErr{"NewPgxStore - CREATE userid index failed - ", err}
	}

	_, err = pdb.Exec(noctx, `CREATE INDEX IF NOT EXISTS `+tableName+`_expires ON `+tableName+` (expires)`)
	if err != nil {
		return st, pgxErr{"NewPgxStore - CREATE userid index failed - ", err}
	}

	return st, nil
}

func (ps *pgxStore) Get(sessIDbytes []byte, uidNOTUSED []byte) ([]byte, []byte, int, int, int, error) {
	sessID := bytesToSessID(sessIDbytes)
	var data, userID []byte
	var ttl, maxage, minrefresh int

	row := ps.db.QueryRow(noctx,
		`SELECT data, userid, FLOOR(EXTRACT(EPOCH FROM (expires-NOW()))), maxage, minrefresh FROM `+
			ps.table+` WHERE id = $1`, sessID)
	if err := row.Scan(&data, &userID, &ttl, &maxage, &minrefresh); err != nil {
		return []byte{}, []byte{}, 0, 0, 0, pgxErr{"pgxStore.Get - row.Scan failed - ", err}
	}

	if ttl <= 0 {
		if _, err := ps.db.Exec(noctx, `DELETE FROM `+ps.table+` WHERE id = $1`, sessID); err != nil {
			return []byte{}, []byte{}, 0, 0, 0, pgxErr{"pgxStore.Get - DELETE failed - ", err}
		}
		return []byte{}, []byte{}, 0, 0, 0, pgxErr{"pgxStore.Get - record has expired", nil}
	}
	return data, userID, ttl, maxage, minrefresh, nil
}

func (ps *pgxStore) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	if *sessID == nil {
		// id is nil: insert a new record and save its id

		var newID uint32

		row := ps.db.QueryRow(noctx, `INSERT INTO `+ps.table+
			` (data, userid, expires, maxage, minrefresh) VALUES($1, $2, NOW() + INTERVAL '`+strconv.Itoa(maxAgeSecs)+` seconds', $3, $4) RETURNING id`,
			data, userID, maxAgeSecs, minRefreshSecs)
		if err := row.Scan(&newID); err != nil {
			return pgxErr{"pgxStore.Save - row.Scan failed - ", err}
		}

		*sessID = sessIDToBytes(newID)
	} else {
		// id is NOT nil: it refers to an existing record; update it.

		cmdtag, err := ps.db.Exec(noctx, `UPDATE `+ps.table+
			` SET data = $1, userid = $2, expires = NOW() + INTERVAL '`+strconv.Itoa(maxAgeSecs)+` seconds', maxage = $3, minrefresh = $4 WHERE id = $5`,
			data, userID, maxAgeSecs, minRefreshSecs, bytesToSessID(*sessID))
		if err != nil {
			return pgxErr{"pgxStore.Save - UPDATE failed - ", err}
		}
		// if record does not exist, UPDATE doesn't return an error! you have to check for no RowsAffected.
		if cmdtag.RowsAffected() < 1 {
			return pgxErr{"pgxStore.Save - UPDATE affected no rows - ", err}
		}
	}
	return nil
}

func (ps *pgxStore) Delete(sessID []byte, uidNOTUSED []byte) error {
	if _, err := ps.db.Exec(noctx, `DELETE FROM `+ps.table+` WHERE id = $1`, bytesToSessID(sessID)); err != nil {
		return pgxErr{"pgxStore.Delete - DELETE failed - ", err}
	}
	return nil
}

func (ps *pgxStore) DeleteByUserID(userID []byte) error {
	if _, err := ps.db.Exec(noctx, `DELETE FROM `+ps.table+` WHERE userid = $1`, userID); err != nil {
		return pgxErr{"pgxStore.DeleteByUserID - DELETE failed - ", err}
	}
	return nil
}

// prune() periodically deletes expired sessions from the session store.
// the "expires" field must be indexed for this to run efficiently.
//
// prune runs in a goroutine, started by NewPgxStore.
// It runs until it receives something on its pruneKill channel.
// You can change its wait interval by sending a number of seconds to its pruneInterval channel.
func (ps *pgxStore) prune(waitSecs int, pruneInterval <-chan int, pruneKill <-chan int, log io.Writer) {
	for {
		select {
		case waitSecs = <-pruneInterval:
		case <-pruneKill:
			return
		case <-time.After(time.Duration(waitSecs) * time.Second):
		}

		ps.db.Exec(noctx, `DELETE FROM `+ps.table+` WHERE expires < NOW()`)
	}
}

// serialize uint32, which we use to store a session id (database key).

func sessIDToBytes(id uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, id)
	return b
}

func bytesToSessID(b []byte) uint32 {
	// could check len(b), but depend on qsess promise not to touch
	return binary.LittleEndian.Uint32(b)
}

type pgxErr struct {
	msg string
	err error
}

func (e pgxErr) Error() string {
	if e.err != nil {
		return "qspgx." + e.msg + " - " + e.err.Error()
	}
	return "qspgx." + e.msg
}
