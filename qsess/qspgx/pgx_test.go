// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qspgx

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	"github.com/jackc/pgx/v5/pgxpool"
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

		pdb, err = pgxpool.New(noctx, url)
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

// TestPgsqlConsume runs the shared ConsumeTokenSession test against PostgreSQL, where the
// concurrent half of it exercises DELETE ... RETURNING under real row locking.
func TestPgsqlConsume(t *testing.T) {
	st := makeTestStore(t, "consume")
	qstest.ConsumeTest(t, st)
	dropTestTable(t, "consume")
}

// TestPgsqlBinding checks that NewPgxStore names a store after its table and that two tables
// sharing a key keep their values apart. After the TRUNCATE ... RESTART IDENTITY the first row in
// each table has id 1, so a value from one table refers to a live row in the other, and only Name
// keeps them apart. Tokens only: the cookie path never reaches the back end and is covered by
// TestBindingCookies in qsess.
func TestPgsqlBinding(t *testing.T) {
	a := makeTestStore(t, "qspgx_binding_a")
	b := makeTestStore(t, "qspgx_binding_b")
	if a.Name != "qspgx_binding_a" || b.Name != "qspgx_binding_b" {
		t.Fatalf("NewPgxStore did not name the stores after their tables: %q, %q", a.Name, b.Name)
	}
	a.AuthType = qsess.TokenAuth
	b.AuthType = qsess.TokenAuth
	if _, err := pdb.Exec(noctx, "TRUNCATE qspgx_binding_a, qspgx_binding_b RESTART IDENTITY"); err != nil {
		t.Fatal("TRUNCATE failed - " + err.Error())
	}

	tokenA := saveBindingToken(t, a)
	saveBindingToken(t, b)
	if _, _, err := a.GetTokenSession(tokenA); err != nil {
		t.Fatal("own token rejected - " + err.Error())
	}
	if _, _, err := b.GetTokenSession(tokenA); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("token bound to one table presented to another: not rejected, err = %v", err)
	}

	// a value issued before Name existed is accepted by default, under the same key, by whichever
	// table receives it, until AcceptUnbound is cleared
	a.Name = ""
	tokenOld := saveBindingToken(t, a)
	saveBindingToken(t, b)
	if _, _, err := b.GetTokenSession(tokenOld); err != nil {
		t.Fatal("unbound token rejected by default - " + err.Error())
	}
	b.AcceptUnbound = false
	if _, _, err := b.GetTokenSession(tokenOld); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("unbound token after AcceptUnbound was cleared: not rejected, err = %v", err)
	}

	dropTestTable(t, "qspgx_binding_a")
	dropTestTable(t, "qspgx_binding_b")
}

// saveBindingToken saves a new session in st and returns its token.
func saveBindingToken(t *testing.T, st *qsess.Store) string {
	sess := st.NewSession([]byte("user"))
	if err := sess.Save(nil); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	token, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}
	return token
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
