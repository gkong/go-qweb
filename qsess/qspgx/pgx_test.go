// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qspgx

import (
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	"github.com/jackc/pgx/v4/pgxpool"
)

var pdb *pgxpool.Pool

const dbName string = "qspgxtest"

// setup
//     sudo -u postgres psql
//     create user qspgx with password 'hello';
//     create database qspgxtest;
//     grant all privileges on database qspgxtest to qspgx;
//
// invoke this program with:
//     DATABASE_URL="postgresql://qspgx:hello@localhost:5432/qspgxtest" go test

func makeTestStore(t *testing.T, tableName string) *qsess.Store {
	var err error

	if pdb == nil {
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			fmt.Fprint(os.Stderr, "no DATABASE_URL environment variable\n", err)
			os.Exit(1)
		}

		pdb, err = pgxpool.Connect(noctx, url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
			os.Exit(1)
		}
	}

	st, err := NewPgxStore(pdb, tableName, os.Stderr, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal("makeTestStore - NewPgxStore failed - " + err.Error())
	}

	return st
}

func dropTestTable(t *testing.T, tableName string) {
	if _, err := pdb.Exec(noctx, "DROP TABLE "+tableName+";"); err != nil {
		t.Fatal("dropTestTable - Exec failed - " + err.Error())
	}
}

func TestPgsqlSanity(t *testing.T) {
	st := makeTestStore(t, "sanity")
	qstest.SanityTest(t, st)
	dropTestTable(t, "sanity")
}

func TestPgsqlNoSessData(t *testing.T) {
	st := makeTestStore(t, "nosessdata")
	qstest.NoSessDataTest(t, st)
	dropTestTable(t, "nosessdata")
}

func TestPgsqlDeleteByUserId(t *testing.T) {
	st := makeTestStore(t, "delbyuid")
	qstest.DeleteByUserIDTest(t, st, true)
	dropTestTable(t, "delbyuid")
}

func TestPgsqlExpiration(t *testing.T) {
	st := makeTestStore(t, "exp")
	qstest.ExpirationTest(t, st)
	dropTestTable(t, "exp")
}

func TestPgsqlSerializer(t *testing.T) {
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
