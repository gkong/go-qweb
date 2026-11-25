// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qspgx

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	"github.com/jackc/pgx/v5"
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

func TestPgsqlErrors(t *testing.T) {
	st := makeTestStore(t, "errors")
	qstest.ErrorsTest(t, st)
	dropTestTable(t, "errors")
}

func TestPgsqlBackEndFailure(t *testing.T) {
	st := makeTestStore(t, "back_end_failure")
	qstest.BackEndFailureTest(t, st, func() { dropTestTable(t, "back_end_failure") })
}

func TestPgsqlSaveAgain(t *testing.T) {
	st := makeTestStore(t, "save_again")
	qstest.SaveAgainTest(t, st)
	dropTestTable(t, "save_again")
}

func TestPgsqlExpiredSave(t *testing.T) {
	st := makeTestStore(t, "expired_save")
	qstest.ExpiredSaveTest(t, st)
	dropTestTable(t, "expired_save")
}

// saveRow saves a row for maxAgeSecs through the back end and returns its id.
func saveRow(t *testing.T, ps *pgxStore, maxAgeSecs int) []byte {
	var id []byte
	if err := ps.Save(&id, []byte{1}, []byte("user"), maxAgeSecs, 0); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	return id
}

// TestPgsqlGetDeletesOnlyExpired checks that Get, given a SELECT that reports every row expired, deletes only a row
// still expired by the time it does, sparing one a Save refreshed after the SELECT, and that it reports a row it
// found expired as a miss even when its DELETE fails.
func TestPgsqlGetDeletesOnlyExpired(t *testing.T) {
	st := makeTestStore(t, "get_delete")
	ps := st.BackEnd().(*pgxStore)
	ps.pGetQuerySQL = `SELECT data, userid, 0, maxage, minrefresh FROM get_delete WHERE id = $1`
	for _, c := range []struct {
		maxAgeSecs int
		gone       bool
	}{{3600, false}, {-1, true}} {
		id := saveRow(t, ps, c.maxAgeSecs)
		if _, _, _, _, _, err := ps.Get(id, nil); !errors.Is(err, qsess.ErrNotFound) {
			t.Fatalf("Get through a SELECT that reports the row expired: err = %v, want a miss", err)
		}
		var one int
		err := ps.db.QueryRow(noctx, "SELECT 1 FROM get_delete WHERE id = $1", bytesToSessID(id)).Scan(&one)
		if gone := errors.Is(err, pgx.ErrNoRows); gone != c.gone {
			t.Errorf("Get of a row saved for %d seconds: gone = %v, want %v - %v", c.maxAgeSecs, gone, c.gone, err)
		}
	}

	ps.pGetDeleteSQL = `DELETE FROM get_delete_missing WHERE id = $1`
	if _, _, _, _, _, err := ps.Get(saveRow(t, ps, -1), nil); !errors.Is(err, qsess.ErrNotFound) {
		t.Errorf("Get of an expired row whose DELETE fails: err = %v, want a miss", err)
	}
	dropTestTable(t, "get_delete")
}

// TestPgsqlExpirySecond checks that Get and Consume report a row as expired once its expires is reached, as Save's
// UPDATE does, and that a row saved for a second is live for that second, which rounding the time left down to
// whole seconds would not let it be.
func TestPgsqlExpirySecond(t *testing.T) {
	st := makeTestStore(t, "expiry_second")
	ps := st.BackEnd().(*pgxStore)

	const expireNow = "UPDATE expiry_second SET expires = NOW() WHERE id = $1"
	for _, c := range []struct {
		name string
		read func([]byte, []byte) ([]byte, []byte, int, int, int, error)
	}{{"Get", ps.Get}, {"Consume", ps.Consume}} {
		if _, _, ttl, _, _, err := c.read(saveRow(t, ps, 1), nil); err != nil || ttl != 1 {
			t.Errorf("%s of a row saved for a second: ttl = %d, err = %v, want 1 and nil", c.name, ttl, err)
		}
		id := saveRow(t, ps, 3600)
		if _, err := ps.db.Exec(noctx, expireNow, bytesToSessID(id)); err != nil {
			t.Fatal("UPDATE failed - " + err.Error())
		}
		if _, _, _, _, _, err := c.read(id, nil); !errors.Is(err, qsess.ErrNotFound) {
			t.Errorf("%s of a row whose expires has been reached: err = %v, want a miss", c.name, err)
		}
	}
	dropTestTable(t, "expiry_second")
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
