// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsmy

import (
	"database/sql"
	"errors"
	"log"
	"math"
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	_ "github.com/go-sql-driver/mysql"
)

var sdb *sql.DB

const dbName string = "qsmytest"

// mysql -u root -p
// create database qsmytest;
// create user 'qsmy'@'localhost' identified with mysql_native_password by 'hello';
// grant event, create, select, insert, update, delete, drop on qsmytest.* to 'qsmy'@'localhost';
// grant super on *.* to 'qsmy'@'localhost';

func makeTestStore(t *testing.T, tableName string) *qsess.Store {
	var err error

	if sdb == nil {
		if sdb, err = sql.Open("mysql", "qsmy:hello@/"+dbName+"?parseTime=true"); err != nil {
			log.Fatalln("sql.Open failed")
		}
	}

	st, err := NewMysqlStore(sdb, tableName, "VARBINARY(600) NOT NULL",
		"VARBINARY(32) NULL",
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal("makeTestStore - NewMysqlStore failed - " + err.Error())
	}

	return st
}

func dropTestTable(t *testing.T, tableName string) {
	_, err := sdb.Exec("DROP TABLE " + tableName + ";")
	if err != nil {
		t.Fatal("dropTestTable - Exec failed - " + err.Error())
	}
}

func TestMysqlSanity(t *testing.T) {
	st := makeTestStore(t, "sanity")
	qstest.SanityTest(t, st)
	dropTestTable(t, "sanity")
}

func TestMysqlNoSessData(t *testing.T) {
	st := makeTestStore(t, "nosessdata")
	qstest.NoSessDataTest(t, st)
	dropTestTable(t, "nosessdata")
}

func TestMysqlDeleteByUserId(t *testing.T) {
	st := makeTestStore(t, "delbyuid")
	qstest.DeleteByUserIDTest(t, st, true)
	dropTestTable(t, "delbyuid")
}

func TestMysqlExpiration(t *testing.T) {
	st := makeTestStore(t, "exp")
	qstest.ExpirationTest(t, st)
	dropTestTable(t, "exp")
}

func TestMysqlErrors(t *testing.T) {
	st := makeTestStore(t, "errors")
	qstest.ErrorsTest(t, st)
	dropTestTable(t, "errors")
}

func TestMysqlBackEndFailure(t *testing.T) {
	st := makeTestStore(t, "back_end_failure")
	qstest.BackEndFailureTest(t, st, func() { dropTestTable(t, "back_end_failure") })
}

// TestMysqlSaveAgain matters for MySQL, which counts the rows an UPDATE changes rather than those it
// matches, so that a second Save within the same second that changes nothing affects no row.
func TestMysqlSaveAgain(t *testing.T) {
	st := makeTestStore(t, "save_again")
	qstest.SaveAgainTest(t, st)
	dropTestTable(t, "save_again")
}

func TestMysqlExpiredSave(t *testing.T) {
	st := makeTestStore(t, "expired_save")
	qstest.ExpiredSaveTest(t, st)
	dropTestTable(t, "expired_save")
}

// saveRow saves a row for maxAgeSecs through the back end and returns its id.
func saveRow(t *testing.T, ss *sqlStore, maxAgeSecs int) []byte {
	var id []byte
	if err := ss.Save(&id, []byte{1}, []byte("user"), maxAgeSecs, 0); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	return id
}

// TestMysqlGetDeletesOnlyExpired checks that Get, given a SELECT that reports every row expired, deletes only a row
// still expired by the time it does, sparing one a Save refreshed after the SELECT, and that it reports a row it
// found expired as a miss even when its DELETE fails.
func TestMysqlGetDeletesOnlyExpired(t *testing.T) {
	st := makeTestStore(t, "get_delete")
	ss := st.BackEnd().(*sqlStore)
	var err error
	ss.sSelect, err = sdb.Prepare("SELECT data, userid, 0, maxage, minrefresh FROM get_delete WHERE id = ?")
	if err != nil {
		t.Fatal("Prepare failed - " + err.Error())
	}
	defer ss.sSelect.Close()
	for _, c := range []struct {
		maxAgeSecs int
		gone       bool
	}{{3600, false}, {-1, true}} {
		id := saveRow(t, ss, c.maxAgeSecs)
		if _, _, _, _, _, err := ss.Get(id, nil); !errors.Is(err, qsess.ErrNotFound) {
			t.Fatalf("Get through a SELECT that reports the row expired: err = %v, want a miss", err)
		}
		var one int
		err := sdb.QueryRow("SELECT 1 FROM get_delete WHERE id = ?", bytesToSessID(id)).Scan(&one)
		if gone := errors.Is(err, sql.ErrNoRows); gone != c.gone {
			t.Errorf("Get of a row saved for %d seconds: gone = %v, want %v - %v", c.maxAgeSecs, gone, c.gone, err)
		}
	}

	ss.sDelExpired.Close() // so that Get's DELETE fails
	if _, _, _, _, _, err := ss.Get(saveRow(t, ss, -1), nil); !errors.Is(err, qsess.ErrNotFound) {
		t.Errorf("Get of an expired row whose DELETE fails: err = %v, want a miss", err)
	}
	dropTestTable(t, "get_delete")
}

// TestMysqlExpirySecond checks that Get and Save report a row as expired from the second its expires names.
func TestMysqlExpirySecond(t *testing.T) {
	st := makeTestStore(t, "expiry_second")
	ss := st.BackEnd().(*sqlStore)
	id := saveRow(t, ss, 3600)
	if _, err := sdb.Exec("UPDATE expiry_second SET expires = NOW() WHERE id = ?", bytesToSessID(id)); err != nil {
		t.Fatal("UPDATE failed - " + err.Error())
	}
	// Save first, as a Get that finds the row expired deletes it
	if err := ss.Save(&id, []byte{1}, []byte("user"), 3600, 0); !errors.Is(err, qsess.ErrNotFound) {
		t.Errorf("Save of a row whose expires has been reached: err = %v, want a miss", err)
	}
	if _, _, _, _, _, err := ss.Get(id, nil); !errors.Is(err, qsess.ErrNotFound) {
		t.Errorf("Get of a row whose expires has been reached: err = %v, want a miss", err)
	}
	dropTestTable(t, "expiry_second")
}

func TestMysqlSerializer(t *testing.T) {
	tests := []uint32{0, 12345, math.MaxUint32}

	for _, n := range tests {
		b := sessIDToBytes(n)
		id := bytesToSessID(b)
		if id != n {
			t.Errorf("ERROR deser(ser(%d)) yields %d\n", n, id)
		}
		// t.Logf("%x %d => %d\n", b, b, intf)
	}
}
