// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qscql

import (
	"flag"
	"os"
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	"github.com/gocql/gocql"
)

const keyspace string = "cqlsesstest"

var db *gocql.Session

func TestMain(m *testing.M) {
	flag.Parse()

	ret := m.Run()

	dbDrop()
	os.Exit(ret)
}

func dbSetup(t *testing.T) {
	if db != nil {
		return
	}

	var err error

	cluster := gocql.NewCluster("127.0.0.1")
	cluster.ProtoVersion = 3
	cluster.Consistency = gocql.Quorum

	// session for creating the keyspace
	db, err = cluster.CreateSession()
	if err != nil {
		t.Fatal("dbSetup - FIRST CreateSession failed - " + err.Error())
	}
	db.Query(`CREATE KEYSPACE IF NOT EXISTS ` + keyspace + ` WITH REPLICATION =
			{ 'class' : 'SimpleStrategy', 'replication_factor' : 1 };`).Exec()
	db.Close()

	// session for on-going operation
	cluster.Keyspace = keyspace
	db, err = cluster.CreateSession()
	if err != nil {
		t.Fatal("dbSetup - SECOND CreateSession failed - " + err.Error())
	}
}

func dbDrop() {
	db.Query("DROP KEYSPACE " + keyspace + ";").Exec()
}

func makeTestStore(t *testing.T, tableName string, uidIndex bool, uidToClient bool) *qsess.Store {
	dbSetup(t)

	st, err := NewCqlStore(db, tableName, uidIndex, uidToClient,
		[]byte("key-to-detect-tampering---------"),
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal("makeTestStore - NewCqlStore failed - " + err.Error())
	}

	return st
}

func TestCassSanity(t *testing.T) {
	st := makeTestStore(t, "sanity", false, false)
	qstest.SanityTest(t, st)
}

func TestCassUCSanity(t *testing.T) {
	st := makeTestStore(t, "UCsanity", false, true)
	qstest.SanityTest(t, st)
}

func TestCassNoSessData(t *testing.T) {
	st := makeTestStore(t, "nosessdata", false, false)
	qstest.NoSessDataTest(t, st)
}

func TestCassUCNoSessData(t *testing.T) {
	st := makeTestStore(t, "UCnosessdata", false, true)
	qstest.NoSessDataTest(t, st)
}

func TestCassDeleteByUserId(t *testing.T) {
	err := db.Query(`CREATE TABLE IF NOT EXISTS "blahdeblah" ( sessid uuid, userid blob, PRIMARY KEY (sessid) )`).Exec()
	if err != nil {
		t.Fatal("CREATE TABLE blahdeblah - " + err.Error())
	}

	err = db.Query(`CREATE INDEX IF NOT EXISTS "blahdeblah_uid_ndx" ON blahdeblah (userid)`).Exec()
	if err != nil {
		t.Log("assuming scylla - skipping this test - because CREATE INDEX failed - " + err.Error())
		return
	}

	st := makeTestStore(t, "delbyuid", true, false)
	qstest.DeleteByUserIDTest(t, st, false)
}

func TestCassUCDeleteByUserId(t *testing.T) {
	st := makeTestStore(t, "UCdelbyuid", false, true)
	qstest.DeleteByUserIDTest(t, st, false)
}

func TestCassExpiration(t *testing.T) {
	st := makeTestStore(t, "exp", false, false)
	qstest.ExpirationTest(t, st)
}
