// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qsmy is a MySQL back-end for qsess.
package qsmy

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/gkong/go-qweb/qsess"
	_ "github.com/go-sql-driver/mysql"
)

// type sqlStore holds per-store information and conforms to the SessBackEnd interface.

type sqlStore struct {
	db          *sql.DB
	sSelect     *sql.Stmt
	sLive       *sql.Stmt
	sInsert     *sql.Stmt
	sUpdate     *sql.Stmt
	sDelete     *sql.Stmt
	sDelExpired *sql.Stmt
	sDelUserID  *sql.Stmt
}

// NewMysqlStore creates a new session store, using a MySQL database.
//
// table is the name of a database table to hold session data (it will be
// created if it doesn't exist). It is also the Store.Name, so stores that
// share keys never accept each other's values (see qsess.Store).
//
// dataField is an SQL column definition for serialized session data,
// for example, "VARBINARY(500) NOT NULL".
//
// uidField is an SQL column definition for user ids.
// It must be indexable and accept []byte values.
//
// cipherkeys are one or more 32-byte encryption keys, to be used with
// AES-GCM. For encryption, only the first key is used;
// for decryption all keys are tried (allowing key rotation).
//
// Additional configuration options can be set by manipulating fields in the
// returned qsess.Store.
func NewMysqlStore(sdb *sql.DB, table string, dataField string, uidField string, cipherkeys ...[]byte) (*qsess.Store, error) {
	ss := &sqlStore{db: sdb}

	st, err := qsess.NewStore(ss, false, cipherkeys...)
	if err != nil {
		return nil, myErr{"NewMysqlStore - NewStore", err}
	}
	st.Name = table

	_, err = sdb.Exec(
		`CREATE TABLE IF NOT EXISTS ` + table + ` (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			data ` + dataField +
			`, userid ` + uidField +
			`, expires DATETIME NOT NULL,
			maxage INT,
			minrefresh INT,
			INDEX(userid),
			INDEX(expires)
		 ) DEFAULT CHARSET=utf8, AUTO_INCREMENT=1;`)
	if err != nil {
		return st, myErr{"NewMysqlStore - CREATE TABLE failed", err}
	}

	_, err = sdb.Exec("SET GLOBAL event_scheduler = ON;")
	if err != nil {
		return st, myErr{"NewMysqlStore - cannot turn on event_scheduler", err}
	}

	_, err = sdb.Exec("CREATE EVENT IF NOT EXISTS prune_expired_qsess ON SCHEDULE EVERY 2 MINUTE DO DELETE FROM " + table + " WHERE expires < NOW();")
	if err != nil {
		return st, myErr{"NewMysqlStore - CREATE EVENT prune_expired_qsess failed", err}
	}

	// calculation of expiration time and test for expiration are done on the MySQL server,
	// so no need for our time to be synchronized with the MySQL server's time.

	ss.sSelect, err = sdb.Prepare(
		`SELECT data, userid, (TIME_TO_SEC(TIMEDIFF(expires,NOW()))), maxage, minrefresh FROM ` +
			table + ` WHERE id = ?`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare SELECT failed", err}
	}

	ss.sLive, err = sdb.Prepare(`SELECT 1 FROM ` + table + ` WHERE id = ? AND expires > NOW()`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare SELECT 1 failed", err}
	}

	ss.sInsert, err = sdb.Prepare(
		`INSERT INTO ` + table +
			` (data, userid, expires, maxage, minrefresh) VALUES(?, ?, ADDTIME(NOW(), SEC_TO_TIME(?)), ?, ?)`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare INSERT failed", err}
	}

	ss.sUpdate, err = sdb.Prepare(
		`UPDATE ` + table +
			` SET data = ?, userid = ?, expires = ADDTIME(NOW(), SEC_TO_TIME(?)), maxage = ?, minrefresh = ?` +
			` WHERE id = ? AND expires > NOW()`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare UPDATE failed", err}
	}

	ss.sDelete, err = sdb.Prepare(`DELETE FROM ` + table + ` WHERE id = ?`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare DELETE failed", err}
	}

	ss.sDelExpired, err = sdb.Prepare(`DELETE FROM ` + table + ` WHERE id = ? AND expires <= NOW()`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare DELETE of an expired row failed", err}
	}

	ss.sDelUserID, err = sdb.Prepare(`DELETE FROM ` + table + ` WHERE userid = ?`)
	if err != nil {
		return st, myErr{"NewMysqlStore - prepare DelUserID failed", err}
	}

	return st, nil
}

func (ss *sqlStore) Get(sessIDbytes []byte, uidNOTUSED []byte) ([]byte, []byte, int, int, int, error) {
	sessID := bytesToSessID(sessIDbytes)
	var data, userID []byte
	var ttl, maxage, minrefresh int
	if err := ss.sSelect.QueryRow(sessID).Scan(&data, &userID, &ttl, &maxage, &minrefresh); err != nil {
		return []byte{}, []byte{}, 0, 0, 0, myErr{"sqlStore.Get - SELECT", notFound(err)}
	}
	if ttl <= 0 {
		// only if still expired, because a Save can refresh the row after the SELECT
		ss.sDelExpired.Exec(sessID)
		return []byte{}, []byte{}, 0, 0, 0, myErr{"sqlStore.Get - record has expired", qsess.ErrNotFound}
	}
	return data, userID, ttl, maxage, minrefresh, nil
}

func (ss *sqlStore) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	if *sessID == nil {
		// id is nil: insert a new record and save its id
		result, err := ss.sInsert.Exec(data, userID, maxAgeSecs, maxAgeSecs, minRefreshSecs)
		if err != nil {
			return myErr{"sqlStore.Save - INSERT failed", err}
		}
		newID, err := result.LastInsertId() // returns an int64
		if err != nil {
			return myErr{"sqlStore.Save - LastInsertId failed", err}
		}
		if newID > int64(math.MaxUint32) {
			return myErr{"sqlStore.Save - returned id is too big", nil}
		}
		*sessID = sessIDToBytes(uint32(newID))
	} else {
		// id is NOT nil: it refers to an existing record; update it.
		id := bytesToSessID(*sessID)
		result, err := ss.sUpdate.Exec(data, userID, maxAgeSecs, maxAgeSecs, minRefreshSecs, id)
		if err != nil {
			return myErr{"sqlStore.Save - UPDATE failed", err}
		}
		if rows, _ := result.RowsAffected(); rows != 1 {
			// NOTE: MySQL counts the rows an UPDATE changes, not those it matches, so a Save that changes
			// nothing, as a second Save within the same second can, affects no row, and only a look tells
			// that from a session that is gone or has expired.
			var found int
			if err := ss.sLive.QueryRow(id).Scan(&found); err != nil {
				return myErr{"sqlStore.Save - SELECT of the live row", notFound(err)}
			}
		}
	}
	return nil
}

func (ss *sqlStore) Delete(sessID []byte, uidNOTUSED []byte) error {
	_, err := ss.sDelete.Exec(bytesToSessID(sessID))
	if err != nil {
		return myErr{"sqlStore.Delete - DELETE failed", err}
	}
	return nil
}

func (ss *sqlStore) DeleteByUserID(userID []byte) error {
	_, err := ss.sDelUserID.Exec(userID)
	if err != nil {
		return myErr{"sqlStore.DeleteByUserID - DELETE failed", err}
	}
	return nil
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

type myErr struct {
	msg string
	err error
}

func (e myErr) Error() string {
	if e.err != nil {
		return "qsmy." + e.msg + " - " + e.err.Error()
	}
	return "qsmy." + e.msg
}

func (e myErr) Unwrap() error {
	return e.err
}

// notFound makes database/sql's no-rows error match qsess.ErrNotFound as well.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w - %w", qsess.ErrNotFound, err)
	}
	return err
}
