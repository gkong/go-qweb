// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsmy

import (
	"database/sql"
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
