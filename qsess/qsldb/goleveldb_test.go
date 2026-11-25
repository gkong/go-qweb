// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsldb

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"math"
	"math/rand"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var testPrefix = []byte{1, 2}

var testGldb *leveldb.DB

var tmpDbName = os.TempDir() + "/" + "0-QSESS-GLDB-TEST-DB"

func TestMain(m *testing.M) {
	flag.Parse()
	ret := m.Run()

	os.RemoveAll(tmpDbName)

	os.Exit(ret)
}

// gldbTestStore opens a goleveldb database, makes a goleveldb session store,
// and sets global testStore to refer to it.
func gldbTestStore(t *testing.T) *qsess.Store {
	var err error
	var st *qsess.Store

	if testGldb == nil {
		if testGldb, err = leveldb.OpenFile(tmpDbName, nil); err != nil {
			t.Fatal("leveldb.OpenFile failed")
		}
	}

	// empty out the database
	iter := testGldb.NewIterator(nil, nil)
	for iter.Next() {
		testGldb.Delete(iter.Key(), nil)
	}
	iter.Release()

	st, err = NewGldbStore(testGldb, testPrefix, os.Stderr,
		[]byte("key-to-detect-tampering---------"),
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal(err)
	}
	st.CookieDomain = "localhost"
	st.CookiePath = "/"
	st.MaxAgeSecs = 3600 * 1 // sessions last one hour
	st.CookieHTTPOnly = true

	return st
}

func TestAccessors(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst, ok := testStore.BackEnd().(*gldbStore)
	if !ok {
		t.Fatal("testStore is not a gldbStore")
	}

	// XXX - TODO - test ALL accessors!

	// expiration index key

	exptime := []byte{7, 7, 7, 7, 7, 7, 7, 7}
	sesskey := make([]byte, sessKeySize(gst.prefixSize))
	for i := range sesskey {
		sesskey[i] = 8
	}

	expkey := gst.expKey(exptime, sesskey)

	if len(expkey) != gst.expKeySize {
		t.Errorf("exp key - bad len - expect %d, got %d", gst.expKeySize, len(expkey))
	}
	if !bytes.Equal(expkey[:len(testPrefix)], testPrefix) {
		t.Error("exp key - bad prefix")
	}
	retrievedExpiration := make([]byte, bytesPerInt64)
	itob(retrievedExpiration, expkey.expiration(gst.prefixSize))
	if !bytes.Equal(retrievedExpiration, exptime) {
		t.Errorf("exp key - bad expiration - expect %x, got %x", exptime, retrievedExpiration)
	}
	if !bytes.Equal(expkey.sessKey(gst.prefixSize), sesskey) {
		t.Error("exp key - bad sess key")
	}
}

func TestGldbBtoi(t *testing.T) {
	key := make([]byte, bytesPerInt64)

	tests := []int64{0, 0x1122334455667788, math.MaxInt64, -1, -12345, (-math.MaxInt64) - 1}

	for _, n := range tests {

		itob(key, n)
		ret := btoi(key)
		if ret != n {
			t.Errorf("ERROR expected %d, got %d\n", n, ret)
		}
		// t.Logf("%x => 0x%x %d\n",  key[:], ret, ret)
	}
}

func TestGldbSanity(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.SanityTest(t, testStore)
}

func TestGldbNoSessData(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.NoSessDataTest(t, testStore)
}

func TestGldbDeleteByUserId(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.DeleteByUserIDTest(t, testStore, true)
}

// TestGldbDeleteByUserIDExact checks that DeleteByUserID deletes the sessions of one user id only.
// The index scan for a user id also finds every longer one that begins with it, which for the empty
// user id is every session in the store.
func TestGldbDeleteByUserIDExact(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	testStore.AuthType = qsess.TokenAuth

	tokens := make(map[string]string)
	for _, userID := range []string{"", "a", "ab", "abc", "b"} {
		sess := testStore.NewSession([]byte(userID))
		if err := sess.Save(nil); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
		token, _, err := sess.Token()
		if err != nil {
			t.Fatal("Token failed - " + err.Error())
		}
		tokens[userID] = token
	}

	deleted := make(map[string]bool)
	for _, userID := range []string{"a", "", "ab"} {
		if err := testStore.NewSession([]byte(userID)).DeleteByUserID(nil); err != nil {
			t.Fatal("DeleteByUserID failed - " + err.Error())
		}
		deleted[userID] = true

		for owner, token := range tokens {
			_, _, err := testStore.GetTokenSession(token)
			if deleted[owner] && err == nil {
				t.Errorf("after DeleteByUserID(%q), user %q still has a session", userID, owner)
			}
			if !deleted[owner] && err != nil {
				t.Errorf("after DeleteByUserID(%q), the session of user %q is gone - %v", userID, owner, err)
			}
		}
	}
}

func TestGldbExpiration(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.ExpirationTest(t, testStore)
}

func TestGldbConsume(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.ConsumeTest(t, testStore)
}

func TestGldbErrors(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.ErrorsTest(t, testStore)
}

func TestGldbSaveAgain(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.SaveAgainTest(t, testStore)
}

// TestGldbSaveRacesRevocation checks that a Save racing a Delete, a DeleteByUserID or a ConsumeTokenSession of
// the same session never leaves it live: whichever runs first, the session ends up gone.
func TestGldbSaveRacesRevocation(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	testStore.AuthType = qsess.TokenAuth

	revocations := []struct {
		what   string
		revoke func(sess *qsess.Session, token string) error
	}{
		{"Delete", func(sess *qsess.Session, token string) error { return sess.Delete(nil) }},
		{"DeleteByUserID", func(sess *qsess.Session, token string) error { return sess.DeleteByUserID(nil) }},
		{"ConsumeTokenSession", func(sess *qsess.Session, token string) error {
			_, err := testStore.ConsumeTokenSession(token)
			return err
		}},
	}
	for _, r := range revocations {
		for trial := 0; trial < 200; trial++ {
			token := gldbSaveToken(t, testStore)
			// two requests that loaded the session: one refreshes it while the other revokes it
			refresher, _, err := testStore.GetTokenSession(token)
			if err != nil {
				t.Fatal("GetTokenSession failed - " + err.Error())
			}
			revoker, _, err := testStore.GetTokenSession(token)
			if err != nil {
				t.Fatal("GetTokenSession failed - " + err.Error())
			}
			refresher.MaxAgeSecs++

			var wg sync.WaitGroup
			start := make(chan struct{})
			var errRevoke error
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				refresher.Save(nil)
			}()
			go func() {
				defer wg.Done()
				<-start
				errRevoke = r.revoke(revoker, token)
			}()
			close(start)
			wg.Wait()

			if errRevoke != nil {
				t.Fatalf("%s failed - %v", r.what, errRevoke)
			}
			if _, _, err := testStore.GetTokenSession(token); err == nil {
				t.Fatalf("trial %d: a Save racing %s left the session live", trial, r.what)
			}
		}
	}
}

func TestGldbExpiredSave(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.ExpiredSaveTest(t, testStore)
}

// TestGldbPruneRacesSave checks that a Save of an expired session fails while a pass of the pruner deletes it, and
// that the pass leaves nothing of the session behind.
func TestGldbPruneRacesSave(t *testing.T) {
	const sessions = 400
	for trial := 0; trial < 10; trial++ {
		db, err := leveldb.OpenFile(t.TempDir(), nil)
		if err != nil {
			t.Fatal("leveldb.OpenFile failed - " + err.Error())
		}
		st, err := NewGldbStore(db, testPrefix, os.Stderr, []byte("key-for-encryption--------------"))
		if err != nil {
			t.Fatal(err)
		}
		gst := st.BackEnd().(*gldbStore)

		keys := make([]gldbSessKey, sessions)
		for i := range keys {
			keys[i] = saveRecord(t, gst, []byte(fmt.Sprint(i)), -1)
		}
		st.PruneInterval <- 3600 // a pass starts at once
		for i := range keys {
			key := bytes.Clone(keys[i])
			err := gst.Save((*[]byte)(&key), []byte{2}, []byte(fmt.Sprint(i)), 3600, 0)
			if !errors.Is(err, qsess.ErrNotFound) {
				t.Fatalf("trial %d: Save of an expired session during a pass: err = %v, want a miss", trial, err)
			}
		}
		st.PruneKill <- 0 // taken once the pass is over

		left := 0
		iter := db.NewIterator(util.BytesPrefix(testPrefix), nil)
		for iter.Next() {
			left++
		}
		iter.Release()
		if left != 0 {
			t.Fatalf("trial %d: a pass left %d records of expired sessions", trial, left)
		}
		db.Close()
	}
}

// TestGldbPruneWaitsForSave checks that a pass of the pruner that finds a session expired waits for a Save that
// read it while it was live, and so leaves the session the Save refreshed, with its index records, in place.
func TestGldbPruneWaitsForSave(t *testing.T) {
	db, err := leveldb.OpenFile(t.TempDir(), nil)
	if err != nil {
		t.Fatal("leveldb.OpenFile failed - " + err.Error())
	}
	defer db.Close()
	st, err := NewGldbStore(db, testPrefix, os.Stderr, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal(err)
	}
	gst := st.BackEnd().(*gldbStore)
	user := []byte("user")

	key := saveRecord(t, gst, user, 1)
	oldExp, err := gst.findExpiration(key)
	if err != nil {
		t.Fatal("findExpiration failed - " + err.Error())
	}

	gst.mu.Lock() // as a Save holds it once it has found the session live
	time.Sleep(2100 * time.Millisecond)
	st.PruneInterval <- 3600           // a pass starts at once and finds the session expired
	time.Sleep(300 * time.Millisecond) // for the pass to wait for mu
	// what the Save then writes
	refreshed, err := newSessValue(len(user), 1)
	if err != nil {
		t.Fatal(err)
	}
	itob(refreshed.expirationBytes(), time.Now().Unix()+3600)
	copy(refreshed.userID(), user)
	newExpKey := gst.expKey(refreshed.expirationBytes(), key)
	for _, err := range []error{db.Put(newExpKey, []byte{}, nil), db.Put(key, refreshed, nil),
		db.Delete(gst.expKey(oldExp, key), nil)} {
		if err != nil {
			t.Fatal("refresh failed - " + err.Error())
		}
	}
	gst.mu.Unlock()
	st.PruneKill <- 0 // taken once the pass is over

	for what, k := range map[string][]byte{"session record": key, "user id index record": gst.uidKey(user, key),
		"expiration index record": newExpKey} {
		if has, _ := db.Has(k, nil); !has {
			t.Errorf("a pass of the pruner deleted the %s of a session a Save refreshed", what)
		}
	}
}

// TestGldbPruneWaitsForFirstSave checks that a pass of the pruner that meets a first Save's expiration index record
// before the Save has written its session record waits for the Save, rather than delete the index record and leave
// the session record without one.
func TestGldbPruneWaitsForFirstSave(t *testing.T) {
	var pausePrefix []byte // the next Put of a key with this prefix waits for resume
	paused := make(chan struct{})
	resume := make(chan struct{})
	st, gst, db := faultStore(t, os.Stderr, func(op string, key []byte) error {
		if op == "Put" && pausePrefix != nil && bytes.HasPrefix(key, pausePrefix) {
			pausePrefix = nil
			close(paused)
			<-resume
		}
		return nil
	})

	pausePrefix = gst.sessPrefix
	saved := make(chan error)
	go func() {
		var key gldbSessKey
		saved <- gst.Save((*[]byte)(&key), []byte{1}, []byte("user"), -1, 0)
	}()
	<-paused                           // the Save has written its expiration index record and no other
	st.PruneInterval <- 3600           // a pass starts at once and finds that record expired
	time.Sleep(300 * time.Millisecond) // for the pass to wait for mu
	close(resume)
	if err := <-saved; err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	st.PruneKill <- 0 // taken once the pass is over
	if n := countKeysWithPrefix(db, testPrefix); n != 0 {
		t.Errorf("the pass left %d records of the expired session, want none", n)
	}
}

// TestGldbPrunerLeavesWhatItCannotIdentify checks that the pruner leaves alone an expired index record naming a
// session record too short for its fields, one naming a key outside this store's session range and one holding a
// value, and logs one line a pass for the first kind and one for the other two, each naming the first such key.
func TestGldbPrunerLeavesWhatItCannotIdentify(t *testing.T) {
	db, err := leveldb.OpenFile(t.TempDir(), nil)
	if err != nil {
		t.Fatal("leveldb.OpenFile failed - " + err.Error())
	}
	defer db.Close()
	var log bytes.Buffer
	st, err := NewGldbStore(db, testPrefix, &log, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal(err)
	}
	gst := st.BackEnd().(*gldbStore)

	expired := make([]byte, bytesPerInt64)
	itob(expired, time.Now().Unix()-60)
	record, err := newSessValue(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	copy(record.expirationBytes(), expired)
	overrun := make([]byte, sessValueFixedPartSize+2)
	overrun[3*bytesPerInt64] = 200 // user id size
	outside := gst.newSessKey()
	outside[gst.prefixSize-1] = 9 // the record type byte

	var kept [][]byte
	put := func(sessKey []byte, value []byte, indexValue []byte) []byte {
		expKey := gst.expKey(expired, sessKey)
		for _, kv := range [][2][]byte{{sessKey, value}, {expKey, indexValue}} {
			if err := db.Put(kv[0], kv[1], nil); err != nil {
				t.Fatal("Put failed - " + err.Error())
			}
		}
		kept = append(kept, sessKey, expKey)
		return expKey
	}
	malformed := [][]byte{put(gst.newSessKey(), []byte{}, []byte{}), put(gst.newSessKey(), []byte{1, 2, 3}, []byte{}),
		put(gst.newSessKey(), overrun, []byte{})}
	foreign := [][]byte{put(outside, record, []byte{}), put(gst.newSessKey(), record, []byte{7})}

	st.PruneInterval <- 3600 // a pass starts at once
	st.PruneKill <- 0        // taken once the pass is over

	for _, key := range kept {
		if has, _ := db.Has(key, nil); !has {
			t.Errorf("the pruner deleted %x, which it cannot identify as this store's", key)
		}
	}
	// the pass meets keys in key order
	for _, want := range []string{
		fmt.Sprintf("2 keys in the expiration index range are not this store's index records, the first %x\n",
			slices.MinFunc(foreign, bytes.Compare)),
		fmt.Sprintf("3 expiration index records name a malformed session record, the first %x\n",
			slices.MinFunc(malformed, bytes.Compare)),
	} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("the pruner did not log %q:\n%s", want, log.String())
		}
	}
	if lines := strings.Count(log.String(), "\n"); lines != 2 {
		t.Errorf("the pruner logged %d lines, want one for each kind:\n%s", lines, log.String())
	}
}

// TestGldbGetRechecksBeforeDeleting checks that Get, having found a session expired, deletes it only under mu and
// only if it is still expired then, so that a Save that refreshed the session in between is not undone.
func TestGldbGetRechecksBeforeDeleting(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst := testStore.BackEnd().(*gldbStore)

	key := saveRecord(t, gst, []byte("u"), -1)
	refreshed, err := newSessValue(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	itob(refreshed.expirationBytes(), time.Now().Unix()+3600)

	gst.mu.Lock()
	got := make(chan error)
	go func() {
		_, _, _, _, _, err := gst.Get(key, nil)
		got <- err
	}()
	time.Sleep(100 * time.Millisecond) // for Get to read the record and wait for mu
	select {
	case <-got:
		gst.mu.Unlock()
		t.Fatal("Get deleted a session it found expired without waiting for mu")
	default:
	}
	// what a Save that refreshed the session meanwhile would have written
	if err := testGldb.Put(key, refreshed, nil); err != nil {
		t.Fatal("Put failed - " + err.Error())
	}
	gst.mu.Unlock()
	if err := <-got; !errors.Is(err, qsess.ErrNotFound) {
		t.Fatalf("Get of a session it read as expired: err = %v, want a miss", err)
	}
	if has, _ := testGldb.Has(key, nil); !has {
		t.Fatal("Get deleted a session that was refreshed after Get found it expired")
	}
}

// TestGldbDeleteByUserIDReportsFailure checks that DeleteByUserID reports a delete that fails, as ErrBackEnd,
// rather than return nil while the user's sessions stay valid, and that an index record whose session is already
// gone is deleted, and is no failure even when deleting it fails.
func TestGldbDeleteByUserIDReportsFailure(t *testing.T) {
	db, err := leveldb.OpenFile(t.TempDir(), nil)
	if err != nil {
		t.Fatal("leveldb.OpenFile failed - " + err.Error())
	}
	defer db.Close()
	st, err := NewGldbStore(db, testPrefix, os.Stderr, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { st.PruneKill <- 0 }()
	gst := st.BackEnd().(*gldbStore)
	user := []byte("user")
	allGone := []byte("all gone")

	gone := saveRecord(t, gst, user, 3600)
	if err := db.Delete(gone, nil); err != nil {
		t.Fatal("Delete failed - " + err.Error())
	}
	live := saveRecord(t, gst, user, 3600)
	liveExp, err := gst.findExpiration(live)
	if err != nil {
		t.Fatal("findExpiration failed - " + err.Error())
	}
	if err := gst.DeleteByUserID(user); err != nil {
		t.Fatal("DeleteByUserID failed - " + err.Error())
	}
	for what, key := range map[string][]byte{
		"session record":                                 live,
		"user id index record":                           gst.uidKey(user, live),
		"expiration index record":                        gst.expKey(liveExp, live),
		"user id index record of a session already gone": gst.uidKey(user, gone),
	} {
		if has, _ := db.Has(key, nil); has {
			t.Errorf("DeleteByUserID left a %s", what)
		}
	}

	saveRecord(t, gst, user, 3600)
	if err := db.Delete(saveRecord(t, gst, allGone, 3600), nil); err != nil {
		t.Fatal("Delete failed - " + err.Error())
	}
	if err := db.SetReadOnly(); err != nil {
		t.Fatal("SetReadOnly failed - " + err.Error())
	}
	if err := st.NewSession(allGone).DeleteByUserID(nil); err != nil {
		t.Fatal("DeleteByUserID of a user whose sessions are all gone, on a read-only database - " + err.Error())
	}
	err = st.NewSession(user).DeleteByUserID(nil)
	qstest.MustMatchClass(t, err, qsess.ErrBackEnd, "DeleteByUserID on a read-only database")
	if !errors.Is(err, leveldb.ErrReadOnly) {
		t.Fatal("goleveldb's own error cannot be reached through errors.Is - " + err.Error())
	}
}

// TestGldbOnlyOwnSessions checks that a store takes a value naming another store's session, as AcceptUnbound or an
// application's own Encrypt and Decrypt can let one through, for one whose session is gone, and that its back end
// neither reads nor changes another store's session, live or expired: one of a store with another prefix, or one
// of a store whose prefix extends this one's, whose session keys begin with this store's session prefix.
func TestGldbOnlyOwnSessions(t *testing.T) {
	a := gldbTestStore(t) // opens and empties the database
	defer func() { a.PruneKill <- 0 }()
	a.AuthType = qsess.TokenAuth
	gstA := a.BackEnd().(*gldbStore)

	for _, prefix := range [][]byte{{3, 4}, bscat(testPrefix, []byte{1})} {
		b := gldbBindingStore(t, prefix)
		gstB := b.BackEnd().(*gldbStore)

		b.Name = "" // a value from before Name existed, which a accepts by default
		token := gldbSaveToken(t, b)
		_, _, err := a.GetTokenSession(token)
		qstest.MustMatchClass(t, err, qsess.ErrInvalidCredential, "GetTokenSession of another store's session")
		_, err = a.ConsumeTokenSession(token)
		qstest.MustMatchClass(t, err, qsess.ErrInvalidCredential, "ConsumeTokenSession of another store's session")

		save := func(maxAgeSecs int) gldbSessKey {
			var key gldbSessKey
			if err := gstB.Save((*[]byte)(&key), []byte{1}, []byte("user"), maxAgeSecs, 0); err != nil {
				t.Fatal("Save failed - " + err.Error())
			}
			return key
		}
		live := save(3600)
		expired := save(-1)
		before := dbContents(t, testGldb)
		for _, key := range []gldbSessKey{live, expired} {
			if _, _, _, _, _, err := gstA.Get(key, nil); !errors.Is(err, qsess.ErrNotFound) {
				t.Errorf("prefix %x: Get of another store's session: err = %v, want a miss", prefix, err)
			}
			id := bytes.Clone(key)
			if err := gstA.Save(&id, []byte{2}, []byte("user"), 3600, 0); !errors.Is(err, qsess.ErrNotFound) {
				t.Errorf("prefix %x: Save of another store's session: err = %v, want a miss", prefix, err)
			}
			if err := gstA.Delete(key, nil); !errors.Is(err, qsess.ErrNotFound) {
				t.Errorf("prefix %x: Delete of another store's session: err = %v, want a miss", prefix, err)
			}
			if _, _, _, _, _, err := gstA.Consume(key, nil); !errors.Is(err, qsess.ErrNotFound) {
				t.Errorf("prefix %x: Consume of another store's session: err = %v, want a miss", prefix, err)
			}
		}
		if !maps.Equal(before, dbContents(t, testGldb)) {
			t.Errorf("prefix %x: a store changed the database through another store's sessions", prefix)
		}
		if _, _, err := b.GetTokenSession(token); err != nil {
			t.Errorf("prefix %x: the session is gone from the store that issued it - %v", prefix, err)
		}
		b.PruneKill <- 0
	}
}

// TestGldbDeleteByUserIDLeavesForeignKeys checks that DeleteByUserID, like the pruner, deletes only keys it can
// identify as this store's index records, which name one of its session keys and hold no value: an application
// key, and the session records of a store whose prefix extends this one's user id index prefix for the user id,
// can have such a record's length, and with the second prefix those records even name one of its session keys.
func TestGldbDeleteByUserIDLeavesForeignKeys(t *testing.T) {
	for _, foreignPrefix := range []string{"a\x03xb", "a\x03xa"} {
		db, err := leveldb.OpenFile(t.TempDir(), nil)
		if err != nil {
			t.Fatal("leveldb.OpenFile failed - " + err.Error())
		}
		key := []byte("key-for-encryption--------------")
		a, err := NewGldbStore(db, []byte("a"), os.Stderr, key)
		if err != nil {
			t.Fatal(err)
		}
		b, err := NewGldbStore(db, []byte(foreignPrefix), os.Stderr, key)
		if err != nil {
			t.Fatal(err)
		}
		gstA := a.BackEnd().(*gldbStore)
		gstB := b.BackEnd().(*gldbStore)

		var own gldbSessKey
		if err := gstA.Save((*[]byte)(&own), []byte{1}, []byte("x"), 3600, 0); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
		foreign := make([][]byte, 3)
		for i := range foreign {
			if err := gstB.Save(&foreign[i], []byte{1}, []byte("y"), 3600, 0); err != nil {
				t.Fatal("Save failed - " + err.Error())
			}
		}
		appKey := bscat(gstA.uidKeyPrefix([]byte("x")), make([]byte, gstA.sessKeySize))
		if err := db.Put(appKey, []byte{}, nil); err != nil {
			t.Fatal("Put failed - " + err.Error())
		}
		if err := gstA.DeleteByUserID([]byte("x")); err != nil {
			t.Fatal("DeleteByUserID failed - " + err.Error())
		}
		if has, _ := db.Has(own, nil); has {
			t.Errorf("prefix %q: DeleteByUserID left the user's own session", foreignPrefix)
		}
		for _, k := range append([][]byte{appKey}, foreign...) {
			if has, _ := db.Has(k, nil); !has {
				t.Errorf("prefix %q: DeleteByUserID deleted %x, another store's or the application's", foreignPrefix, k)
			}
		}
		a.PruneKill <- 0
		b.PruneKill <- 0
		db.Close()
	}
}

// TestGldbOversizeUserID checks that a user id too long for a session record is ErrApplication, as documented
// misuse, and that a first Save that fails so writes nothing and leaves the session unsaved, so that a second
// fails the same way rather than as a session that is gone.
func TestGldbOversizeUserID(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	testStore.AuthType = qsess.TokenAuth

	sess := testStore.NewSession(make([]byte, 256))
	qstest.MustMatchClass(t, sess.Save(nil), qsess.ErrApplication, "Save with a 256-byte user id")
	qstest.MustMatchClass(t, sess.Save(nil), qsess.ErrApplication, "a second Save with a 256-byte user id")
	_, _, err := sess.Token()
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "Token of a session whose Save failed")
	if n := countKeysWithPrefix(testGldb, testPrefix); n != 0 {
		t.Fatalf("a Save that failed wrote %d records", n)
	}
}

// TestGldbBackEndErrors checks that a session record that cannot be decoded, being too short or too short
// for the user id it claims, and a closed database are reported as qsess.ErrBackEnd and not as a miss.
func TestGldbBackEndErrors(t *testing.T) {
	db, err := leveldb.OpenFile(t.TempDir(), nil)
	if err != nil {
		t.Fatal("leveldb.OpenFile failed - " + err.Error())
	}
	defer db.Close()
	st, err := NewGldbStore(db, testPrefix, os.Stderr, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { st.PruneKill <- 0 }()
	st.AuthType = qsess.TokenAuth
	gst := st.BackEnd().(*gldbStore)

	sess := st.NewSession([]byte("user"))
	if err := sess.Save(nil); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	token, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}

	var sessKeys [][]byte
	iter := db.NewIterator(util.BytesPrefix(gst.sessPrefix), nil)
	for iter.Next() {
		sessKeys = append(sessKeys, bytes.Clone(iter.Key()))
	}
	iter.Release()
	if len(sessKeys) != 1 {
		t.Fatalf("found %d session records, want 1", len(sessKeys))
	}
	overrun := make([]byte, sessValueFixedPartSize+2)
	itob(overrun[:bytesPerInt64], time.Now().Unix()+3600)
	overrun[3*bytesPerInt64] = 200 // user id size
	for what, record := range map[string][]byte{"too short": {1, 2, 3}, "too short for its user id": overrun} {
		if err := db.Put(sessKeys[0], record, nil); err != nil {
			t.Fatal("Put failed - " + err.Error())
		}
		_, _, err = st.GetTokenSession(token)
		qstest.MustMatchClass(t, err, qsess.ErrBackEnd, "session record "+what)
		_, err = st.ConsumeTokenSession(token)
		qstest.MustMatchClass(t, err, qsess.ErrBackEnd, "consuming a session record "+what)
		qstest.MustMatchClass(t, sess.Delete(nil), qsess.ErrBackEnd, "deleting a session record "+what)
		qstest.MustMatchClass(t, sess.Save(nil), qsess.ErrBackEnd, "saving a session record "+what)
		if got, _ := db.Get(sessKeys[0], nil); !bytes.Equal(got, record) {
			t.Fatal("Save overwrote a session record " + what)
		}
	}

	qstest.BackEndFailureTest(t, st, func() { db.Close() })
	_, _, err = st.GetTokenSession(token)
	if !errors.Is(err, leveldb.ErrClosed) {
		t.Fatal("goleveldb's own error cannot be reached through errors.Is - " + err.Error())
	}
	qstest.MustMatchClass(t, sess.DeleteByUserID(nil), qsess.ErrBackEnd, "DeleteByUserID on a closed database")
	if err := sess.Save(nil); !errors.Is(err, leveldb.ErrClosed) {
		t.Fatalf("Save on a closed database: goleveldb's own error cannot be reached through errors.Is - err = %v", err)
	}
}

// TestGldbSaveKeepsExpireIndex checks that a session saved again is still in the expiration index,
// under its new expiration time only. A second Save within one second leaves the index key as it
// was, so there is no old record to delete.
func TestGldbSaveKeepsExpireIndex(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst, ok := testStore.BackEnd().(*gldbStore)
	if !ok {
		t.Fatal("testStore is not a gldbStore")
	}

	var key gldbSessKey
	var firstExp []byte
	var secondExp []byte
	sameSecond := false
	for try := 0; try < 10 && !sameSecond; try++ {
		key = nil
		firstExp = gldbSaveExpiration(t, gst, &key)
		secondExp = gldbSaveExpiration(t, gst, &key)
		sameSecond = bytes.Equal(firstExp, secondExp)
	}
	if !sameSecond {
		t.Fatal("could not save a session twice within one second")
	}
	if _, err := testGldb.Get(gst.expKey(secondExp, key), nil); err != nil {
		t.Fatal("session saved twice within one second is not in the expiration index - " + err.Error())
	}

	time.Sleep(1100 * time.Millisecond)

	thirdExp := gldbSaveExpiration(t, gst, &key)
	if _, err := testGldb.Get(gst.expKey(thirdExp, key), nil); err != nil {
		t.Fatal("session saved again later is not in the expiration index - " + err.Error())
	}
	if _, err := testGldb.Get(gst.expKey(secondExp, key), nil); err == nil {
		t.Fatal("session saved again later is still in the expiration index under its old expiration time")
	}
}

// saveRecord saves a session for maxAgeSecs through the back end and returns its key.
func saveRecord(t *testing.T, gst *gldbStore, user []byte, maxAgeSecs int) gldbSessKey {
	var key gldbSessKey
	if err := gst.Save((*[]byte)(&key), []byte{1}, user, maxAgeSecs, 0); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	return key
}

// given a session key, read its session record and return its expiration time
func (gst *gldbStore) findExpiration(sessKey gldbSessKey) ([]byte, error) {
	data, err := gst.db.Get(sessKey, nil)
	if err != nil {
		return []byte{}, gldbErr{"gldbStore.findExpiration - Get", err}
	}
	if len(data) < sessValueFixedPartSize {
		return []byte{}, gldbErr{"gldbStore.findExpiration - malformed session record", nil}
	}
	sessVal := gldbSessValue(data)
	return sessVal.expirationBytes(), nil
}

// gldbSaveExpiration saves a session through the back end and returns its expiration time as stored.
func gldbSaveExpiration(t *testing.T, gst *gldbStore, key *gldbSessKey) []byte {
	if err := gst.Save((*[]byte)(key), []byte{1, 2, 3}, []byte{4, 5, 6}, 1800, 0); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	exp, err := gst.findExpiration(*key)
	if err != nil {
		t.Fatal("findExpiration failed - " + err.Error())
	}
	return exp
}

func TestGldbExpireIndex(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst, ok := testStore.BackEnd().(*gldbStore)
	if !ok {
		t.Fatal("testStore is not a gldbStore")
	}

	// expireTest without pruner
	expireTest(t, testStore, gst, false)

	// put a record of the wrong length into this store's expiration index range, for pruner
	badKey := bscat(gst.expPrefix, []byte{4, 5, 6})
	testGldb.Put(badKey, []byte{}, nil)
	if _, err := gst.db.Get(badKey, nil); err != nil {
		t.Error("bad key not inserted")
	}

	// expireTest with pruner
	expireTest(t, testStore, gst, true)

	// The pruner must leave it alone. A key of the wrong length in this range can equally be
	// another store's or an application's, and nothing in the key says which.
	if _, err := testGldb.Get(badKey, nil); err != nil {
		t.Error("key of the wrong length was pruned, though it cannot be proved to be this store's")
	}
}

// TestGldbPrunerLeavesForeignKeys checks that one store's pruner does not delete another store's
// records. A second store whose prefix extends this store's expiration prefix writes every one of
// its keys into this store's expiration index range, which NewGldbStore's doc comment invites by
// offering the prefix as the way for session data to coexist with other data in one database.
func TestGldbPrunerLeavesForeignKeys(t *testing.T) {
	storeA := gldbTestStore(t)
	defer func() { storeA.PruneKill <- 0 }()

	// storeB's prefix is storeA's prefix plus the expiration type byte, so every key storeB
	// writes begins with storeA's expPrefix.
	foreignPrefix := bscat(testPrefix, []byte{2})
	storeB, err := NewGldbStore(testGldb, foreignPrefix, os.Stderr,
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal("NewGldbStore for the second store failed - " + err.Error())
	}
	defer func() { storeB.PruneKill <- 0 }()
	gstB := storeB.BackEnd().(*gldbStore)

	var keyB gldbSessKey
	if err := gstB.Save((*[]byte)(&keyB), []byte{9}, []byte("userB"), 3600, 0); err != nil {
		t.Fatal("Save into the second store failed - " + err.Error())
	}

	before := countKeysWithPrefix(testGldb, foreignPrefix)
	if before == 0 {
		t.Fatal("the second store wrote no keys")
	}

	storeA.PruneInterval <- 1
	time.Sleep(3 * time.Second)

	if after := countKeysWithPrefix(testGldb, foreignPrefix); after != before {
		t.Errorf("storeA's pruner deleted the second store's records - %d keys before, %d after",
			before, after)
	}
	if _, _, _, _, _, err := gstB.Get(keyB, nil); err != nil {
		t.Errorf("the second store can no longer read its own session - %v", err)
	}
}

func countKeysWithPrefix(db *leveldb.DB, prefix []byte) int {
	n := 0
	iter := db.NewIterator(nil, nil)
	for ok := iter.Seek(prefix); ok; ok = iter.Next() {
		if !bytes.HasPrefix(iter.Key(), prefix) {
			break
		}
		n++
	}
	iter.Release()
	return n
}

// dbContents returns every key in db with its value.
func dbContents(t *testing.T, db *leveldb.DB) map[string]string {
	contents := make(map[string]string)
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		contents[string(iter.Key())] = string(iter.Value())
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		t.Fatal("iterator failed - " + err.Error())
	}
	return contents
}

func expireTest(t *testing.T, testStore *qsess.Store, gst *gldbStore, usePruner bool) {
	var key gldbSessKey

	if usePruner {
		testStore.PruneInterval <- 1
	}

	// make a record with time-to-live = 2 sec
	if err := gst.Save((*[]byte)(&key), []byte{1, 2, 3}, []byte{4, 5, 6}, 2, 3); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	// immediate Get should succeed, since expiration not yet reached
	_, userID, ttl, _, _, err := gst.Get(key, []byte{})
	if err != nil || ttl < 0 || ttl > 2 {
		t.Fatal("problem with first Get")
	}
	exp, err := gst.findExpiration(key)
	if err != nil {
		t.Fatal("problem with findExpiration")
	}
	// should find entry in expiration index
	wholeExpKey := gst.expKey(exp, key)
	if _, err := testGldb.Get(wholeExpKey, nil); err != nil {
		t.Fatal("record not found in expiration index")
	}
	// should find entry in user id index
	ukey := gst.uidKey(userID, key)
	has, err := testGldb.Has(ukey, nil)
	if err != nil || !has {
		t.Fatal("record not found in userid index")
	}

	// sleep until after expiration time
	time.Sleep(5 * time.Second)

	if !usePruner {
		// Get should notice the session is expired, delete it, and return an error
		if _, _, ttl, _, _, err = gst.Get(key, []byte{}); err == nil {
			t.Fatal("Get succeeded; should have failed due to expiration")
		}
	}

	// peek at the database to confirm the session record is gone
	if _, err := testGldb.Get(key[:], nil); err == nil {
		t.Fatal("session record still there; should have been deleted")
	}
	// entry in expiration index should be gone
	if _, err := testGldb.Get(wholeExpKey, nil); err == nil {
		t.Fatal("record found in expiration index; should have been deleted")
	}
	// entry in user id index should be gone
	has, err = testGldb.Has(ukey, nil)
	if has {
		t.Fatal("record found in userid index; should have been deleted")
	}
}

// TestGldbPruneUnordered checks that the pruner reaches expired sessions whose expiration index
// records sort behind those of unexpired ones. The long-lived sessions expire in more than 256
// consecutive seconds, so that some of their records sort first whatever the clock says.
func TestGldbPruneUnordered(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst, ok := testStore.BackEnd().(*gldbStore)
	if !ok {
		t.Fatal("testStore is not a gldbStore")
	}

	longLived := make([]gldbSessKey, 260)
	for i := range longLived {
		if err := gst.Save((*[]byte)(&longLived[i]), []byte{1, 2, 3}, []byte{4, 5, 6}, 3600+i, 0); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
	}
	// two of them, expiring in different seconds, because one alone could happen to sort first
	expiring := make([]gldbSessKey, 2)
	for i := range expiring {
		if err := gst.Save((*[]byte)(&expiring[i]), []byte{1, 2, 3}, []byte{4, 5, 6}, 1+i, 0); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
	}

	testStore.PruneInterval <- 1

	deadline := time.Now().Add(10 * time.Second)
	for {
		remaining := 0
		for _, key := range expiring {
			if has, _ := testGldb.Has(key, nil); has {
				remaining++
			}
		}
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pruner left %d of %d expired sessions", remaining, len(expiring))
		}
		time.Sleep(200 * time.Millisecond)
	}

	for _, key := range longLived {
		if has, _ := testGldb.Has(key, nil); !has {
			t.Fatal("pruner deleted a session that has not expired")
		}
	}
}

// TestGldbBinding checks that NewGldbStore names a store after its prefix and that, of two stores sharing a database
// and a key list, one refuses the other's values: a bound one does not decrypt, and an unbound one, which it decrypts
// while AcceptUnbound is set, names a session that is not its own. Tokens only: the cookie path never reaches the
// back end and is covered by TestBindingCookies in qsess.
func TestGldbBinding(t *testing.T) {
	a := gldbTestStore(t) // opens and empties the database
	defer func() { a.PruneKill <- 0 }()
	a.AuthType = qsess.TokenAuth

	bPrefix := []byte{3, 4} // same length as testPrefix, so both stores use one key size
	b := gldbBindingStore(t, bPrefix)
	defer func() { b.PruneKill <- 0 }()

	if a.Name != string(testPrefix) || b.Name != string(bPrefix) {
		t.Fatalf("NewGldbStore did not name the stores after their prefixes: %q, %q", a.Name, b.Name)
	}

	tokenA := gldbSaveToken(t, a)
	if _, _, err := a.GetTokenSession(tokenA); err != nil {
		t.Fatal("own token rejected - " + err.Error())
	}
	if _, _, err := b.GetTokenSession(tokenA); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("token bound to one prefix presented to another: not rejected, err = %v", err)
	}

	// a value issued before Name existed carries no binding, so a store accepts its own such
	// values by default, until AcceptUnbound is cleared, and decrypts another store's, but
	// then finds that the session it names is not its own
	a.Name = ""
	tokenUnbound := gldbSaveToken(t, a)
	a.Name = string(testPrefix)
	if _, _, err := a.GetTokenSession(tokenUnbound); err != nil {
		t.Fatal("own unbound token rejected by default - " + err.Error())
	}
	_, _, err := b.GetTokenSession(tokenUnbound)
	qstest.MustMatchClass(t, err, qsess.ErrInvalidCredential, "another store's unbound token")
	if !strings.Contains(err.Error(), "not this store's session") {
		t.Fatalf("another store's unbound token: not decrypted while AcceptUnbound is set, err = %v", err)
	}
	a.AcceptUnbound = false
	if _, _, err := a.GetTokenSession(tokenUnbound); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("unbound token after AcceptUnbound was cleared: not rejected, err = %v", err)
	}
}

// gldbBindingStore makes another session store in the test database, with its own prefix and the
// same keys, leaving the database as it is.
func gldbBindingStore(t *testing.T, prefix []byte) *qsess.Store {
	st, err := NewGldbStore(testGldb, prefix, os.Stderr,
		[]byte("key-to-detect-tampering---------"),
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal(err)
	}
	st.MaxAgeSecs = 3600 * 1
	st.AuthType = qsess.TokenAuth

	return st
}

// gldbSaveToken saves a new session in st and returns its token.
func gldbSaveToken(t *testing.T, st *qsess.Store) string {
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

// faultDB passes each call through to the database, except a Get, Put or Delete for which fail returns an error.
type faultDB struct {
	*leveldb.DB
	fail func(op string, key []byte) error
}

func (f faultDB) Get(key []byte, ro *opt.ReadOptions) ([]byte, error) {
	if err := f.fail("Get", key); err != nil {
		return nil, err
	}
	return f.DB.Get(key, ro)
}

func (f faultDB) Put(key, value []byte, wo *opt.WriteOptions) error {
	if err := f.fail("Put", key); err != nil {
		return err
	}
	return f.DB.Put(key, value, wo)
}

func (f faultDB) Delete(key []byte, wo *opt.WriteOptions) error {
	if err := f.fail("Delete", key); err != nil {
		return err
	}
	return f.DB.Delete(key, wo)
}

// NewIterator's iterator visits every key, and then its Error reports what fail returns for op "Iterator".
func (f faultDB) NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator {
	return faultIterator{f.DB.NewIterator(slice, ro), f.fail("Iterator", nil)}
}

type faultIterator struct {
	iterator.Iterator
	err error
}

func (i faultIterator) Error() error {
	if i.err != nil {
		return i.err
	}
	return i.Iterator.Error()
}

var errInjected = errors.New("injected failure")

// faultStore makes a token store on a new database, logging to errLog, whose calls fail where fail says. It wraps the
// database before any pass of the pruner, which reads gst.db only once a value is sent on PruneInterval or two
// minutes have passed.
func faultStore(t *testing.T, errLog io.Writer,
	fail func(op string, key []byte) error) (*qsess.Store, *gldbStore, *leveldb.DB) {
	db, err := leveldb.OpenFile(t.TempDir(), nil)
	if err != nil {
		t.Fatal("leveldb.OpenFile failed - " + err.Error())
	}
	t.Cleanup(func() { db.Close() })
	st, err := NewGldbStore(db, testPrefix, errLog, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal(err)
	}
	st.AuthType = qsess.TokenAuth
	st.MaxAgeSecs = 3600
	gst := st.BackEnd().(*gldbStore)
	gst.db = faultDB{db, fail}
	return st, gst, db
}

// TestGldbIndexDeleteFailures checks that Delete and Consume succeed when they delete the session record and only
// deleting an index record fails, since the session is gone and the next DeleteByUserID, or pass of the pruner,
// deletes what is left.
func TestGldbIndexDeleteFailures(t *testing.T) {
	var failKey []byte
	failures := 0
	st, gst, db := faultStore(t, os.Stderr, func(op string, key []byte) error {
		if op == "Delete" && bytes.Equal(key, failKey) {
			failures++
			return errInjected
		}
		return nil
	})
	defer func() { st.PruneKill <- 0 }()
	user := []byte("user")

	for _, index := range []string{"user id", "expiration"} {
		for _, call := range []string{"Delete", "Consume"} {
			key := saveRecord(t, gst, user, 3600)
			exp, err := gst.findExpiration(key)
			if err != nil {
				t.Fatal("findExpiration failed - " + err.Error())
			}
			failKey = gst.uidKey(user, key)
			if index == "expiration" {
				failKey = gst.expKey(exp, key)
			}
			before := failures
			var data []byte
			if call == "Delete" {
				err = gst.Delete(key, nil)
			} else {
				data, _, _, _, _, err = gst.Consume(key, nil)
			}
			if failures == before {
				t.Fatalf("%s did not delete the %s index record", call, index)
			}
			if err != nil {
				t.Errorf("%s, the %s index record's delete failing: err = %v, want nil", call, index, err)
			}
			if call == "Consume" && !bytes.Equal(data, []byte{1}) {
				t.Errorf("Consume, the %s index record's delete failing: data = %x, want the session's", index, data)
			}
			if has, _ := db.Has(key, nil); has {
				t.Errorf("%s, the %s index record's delete failing: the session record is left", call, index)
			}
		}
	}

	failKey = nil
	if err := gst.DeleteByUserID(user); err != nil {
		t.Fatal("DeleteByUserID failed - " + err.Error())
	}
	if n := countKeysWithPrefix(db, gst.uidPrefix); n != 0 {
		t.Errorf("DeleteByUserID left %d user id index records of sessions already gone", n)
	}
}

// TestGldbDeleteByUserIDCarriesOn checks that DeleteByUserID, when it cannot delete one of a user's session records,
// still deletes the user's other sessions, one whose record cannot be read among them, reports the first failure,
// though its scan then fails too, and keeps the user id index record of each session it leaves, so that a second
// call finishes the job.
func TestGldbDeleteByUserIDCarriesOn(t *testing.T) {
	failing := make(map[string]string) // a session key, or none for the scan, and the call that fails for it
	failed := make(map[string]int)     // the failures injected, by call
	st, gst, db := faultStore(t, os.Stderr, func(op string, key []byte) error {
		if failing[string(key)] == op {
			failed[op]++
			return fmt.Errorf("injected failure of %s %x", op, key)
		}
		return nil
	})
	defer func() { st.PruneKill <- 0 }()
	user := []byte("user")

	keys := make([]gldbSessKey, 5)
	for i := range keys {
		keys[i] = saveRecord(t, gst, user, 3600)
	}
	slices.SortFunc(keys, func(a, b gldbSessKey) int { return bytes.Compare(a, b) }) // the scan's order
	failing[string(keys[1])] = "Delete"
	failing[string(keys[2])] = "Get"
	failing[string(keys[3])] = "Delete"
	failing[""] = "Iterator"

	err := st.NewSession(user).DeleteByUserID(nil)
	if want := map[string]int{"Delete": 2, "Get": 1, "Iterator": 1}; !maps.Equal(failed, want) {
		t.Fatalf("DeleteByUserID met the failures %v, want %v", failed, want)
	}
	qstest.MustMatchClass(t, err, qsess.ErrBackEnd, "DeleteByUserID that could not delete two session records")
	if !strings.Contains(err.Error(), fmt.Sprintf("%x", keys[1])) {
		t.Errorf("DeleteByUserID reported a failure other than the first - %v", err)
	}
	for i, key := range keys {
		fails := failing[string(key)]
		left := fails == "Delete"
		if has, _ := db.Has(key, nil); has != left {
			t.Errorf("session %d, failing %q: session record left = %v, want %v", i, fails, has, left)
		}
		if has, _ := db.Has(gst.uidKey(user, key), nil); has != left {
			t.Errorf("session %d, failing %q: user id index record left = %v, want %v", i, fails, has, left)
		}
	}

	clear(failing)
	if err := st.NewSession(user).DeleteByUserID(nil); err != nil {
		t.Fatal("a second DeleteByUserID failed - " + err.Error())
	}
	for i, key := range keys {
		if has, _ := db.Has(key, nil); has {
			t.Errorf("session %d is left after a second DeleteByUserID", i)
		}
	}
}

// TestGldbSaveOldIndexDeleteFails checks that a Save that refreshed a session succeeds when only deleting the old
// expiration index record fails, as the refresh took effect, and that the pruner deletes that record at its
// expiration time and leaves the session.
func TestGldbSaveOldIndexDeleteFails(t *testing.T) {
	var failKey []byte
	failures := 0
	st, gst, db := faultStore(t, os.Stderr, func(op string, key []byte) error {
		if op == "Delete" && bytes.Equal(key, failKey) {
			failures++
			return errInjected
		}
		return nil
	})
	user := []byte("user")

	// two seconds, so that the refresh finds the session live unless more than a second passes in between
	key := saveRecord(t, gst, user, 2)
	oldExp, err := gst.findExpiration(key)
	if err != nil {
		t.Fatal("findExpiration failed - " + err.Error())
	}
	failKey = gst.expKey(oldExp, key)
	if err := gst.Save((*[]byte)(&key), []byte{2}, user, 3600, 0); err != nil {
		t.Fatal("a refresh whose delete of the old expiration index record failed - " + err.Error())
	}
	if failures != 1 {
		t.Fatalf("the refresh deleted the old expiration index record %d times, want 1", failures)
	}
	newExp, err := gst.findExpiration(key)
	if err != nil {
		t.Fatal("findExpiration failed - " + err.Error())
	}

	failKey = nil
	time.Sleep(3 * time.Second) // until the old expiration time has passed
	st.PruneInterval <- 3600    // a pass starts at once
	st.PruneKill <- 0           // taken once the pass is over
	if has, _ := db.Has(gst.expKey(oldExp, key), nil); has {
		t.Error("the pruner left the old expiration index record")
	}
	for what, k := range map[string][]byte{"session record": key, "user id index record": gst.uidKey(user, key),
		"expiration index record": gst.expKey(newExp, key)} {
		if has, _ := db.Has(k, nil); !has {
			t.Errorf("the pruner deleted the %s of the refreshed session", what)
		}
	}
}

// TestGldbPrunerRetries checks that a pass of the pruner that cannot read or delete an expired session record keeps
// the expiration index record that names it, so that a later pass deletes the session, and logs in one line how many
// such records it kept, the first of them and its failure.
func TestGldbPrunerRetries(t *testing.T) {
	failing := make(map[string]bool)                  // session keys whose calls fail
	failures := map[string]int{"Get": 2, "Delete": 2} // calls for those keys still to fail
	var errLog bytes.Buffer
	st, gst, db := faultStore(t, &errLog, func(op string, key []byte) error {
		if failing[string(key)] && failures[op] > 0 {
			failures[op]--
			return fmt.Errorf("injected failure of %s %x", op, key)
		}
		return nil
	})

	keys := make([]gldbSessKey, 2)
	eKeys := make([][]byte, 2)
	for i := range keys {
		keys[i] = saveRecord(t, gst, []byte("user"), -1)
		exp, err := gst.findExpiration(keys[i])
		if err != nil {
			t.Fatal("findExpiration failed - " + err.Error())
		}
		eKeys[i] = gst.expKey(exp, keys[i])
	}
	for _, key := range keys {
		failing[string(key)] = true // only now, as findExpiration reads the session record
	}
	// a pass whose reads fail, one whose deletes fail, and one that deletes the sessions
	for range 3 {
		st.PruneInterval <- 3600
	}
	st.PruneKill <- 0 // taken once the last pass is over
	if failures["Get"] != 0 || failures["Delete"] != 0 {
		t.Fatalf("the passes did not meet every failure, %v left", failures)
	}
	if n := countKeysWithPrefix(db, testPrefix); n != 0 {
		t.Fatalf("three passes left %d records of expired sessions", n)
	}
	first := gldbExpKey(slices.MinFunc(eKeys, bytes.Compare)) // the pass meets keys in key order
	var want string
	for _, failure := range []string{"gldbStore.pruneSession - Get - injected failure of Get",
		"gldbStore.deleteSession - session Delete - injected failure of Delete"} {
		want += fmt.Sprintf("qsldb.prune - 2 expiration index records name a session record that could not be read "+
			"or deleted, the first %x - qsldb.%s %x\n", first, failure, first.sessKey(gst.prefixSize))
	}
	if errLog.String() != want {
		t.Errorf("the passes logged:\n%swant:\n%s", errLog.String(), want)
	}
}

// TestGldbPrunerNilErrLog checks that a pass of the pruner of a store whose errLog is nil, which it must not write to,
// leaves each kind of record that it logs.
func TestGldbPrunerNilErrLog(t *testing.T) {
	var failKey []byte // a Get of this key fails
	st, gst, db := faultStore(t, nil, func(op string, key []byte) error {
		if op == "Get" && failKey != nil && bytes.Equal(key, failKey) {
			return errInjected
		}
		return nil
	})

	failKey = saveRecord(t, gst, []byte("user"), -1)
	expired := make([]byte, bytesPerInt64)
	itob(expired, time.Now().Unix()-60)
	malformed := gst.newSessKey()
	for _, kv := range [][2][]byte{{malformed, {}}, {gst.expKey(expired, malformed), {}},
		{gst.expKey(expired, gst.newSessKey()), {7}}} {
		if err := db.Put(kv[0], kv[1], nil); err != nil {
			t.Fatal("Put failed - " + err.Error())
		}
	}

	st.PruneInterval <- 3600 // a pass starts at once
	st.PruneKill <- 0        // taken once the pass is over
	// the unreadable session's three records, the malformed one with its index record, and the foreign index record
	if n := countKeysWithPrefix(db, testPrefix); n != 6 {
		t.Errorf("the pass left %d records, want 6", n)
	}
}

// TestGldbFirstSaveFails checks that a first Save that fails at any of its writes leaves the session unsaved, so that
// Token refuses it and the next Save makes the session anew, and leaves no session record without the expiration
// index record that the pruner reclaims it by.
func TestGldbFirstSaveFails(t *testing.T) {
	var failPrefix []byte // a Put of a key with this prefix fails
	st, gst, db := faultStore(t, os.Stderr, func(op string, key []byte) error {
		if op == "Put" && failPrefix != nil && bytes.HasPrefix(key, failPrefix) {
			return errInjected
		}
		return nil
	})
	defer func() { st.PruneKill <- 0 }()

	for _, c := range []struct {
		write  string
		prefix []byte
		left   int // session records that a first Save failing at this write leaves
	}{{"expiration index", gst.expPrefix, 0}, {"session", gst.sessPrefix, 0}, {"user id index", gst.uidPrefix, 1}} {
		before := countKeysWithPrefix(db, gst.sessPrefix)
		failPrefix = c.prefix
		sess := st.NewSession([]byte("user"))
		qstest.MustMatchClass(t, sess.Save(nil), qsess.ErrBackEnd, "a first Save whose "+c.write+" write failed")
		_, _, err := sess.Token()
		qstest.MustMatchClass(t, err, qsess.ErrApplication, "Token of a session whose first Save failed at its "+
			c.write+" write")
		if n := countKeysWithPrefix(db, gst.sessPrefix) - before; n != c.left {
			t.Errorf("a first Save whose %s write failed left %d session records, want %d", c.write, n, c.left)
		}
		iter := db.NewIterator(util.BytesPrefix(gst.sessPrefix), nil)
		for iter.Next() {
			value := gldbSessValue(iter.Value())
			if has, _ := db.Has(gst.expKey(value.expirationBytes(), iter.Key()), nil); !has {
				t.Errorf("a first Save whose %s write failed left session record %x with no expiration index record",
					c.write, iter.Key())
			}
		}
		iter.Release()

		failPrefix = nil
		if err := sess.Save(nil); err != nil {
			t.Fatal("Save after a first Save that failed - " + err.Error())
		}
		if _, _, err := sess.Token(); err != nil {
			t.Fatal("Token of a session saved after a first Save that failed - " + err.Error())
		}
	}
}

// TestGldbSessionDeleteFails checks that Delete and ConsumeTokenSession report a failure to delete the session record
// as ErrBackEnd, as the session stays valid: the deleted one can still be read, and the consumed one's token is
// redeemed once, by the next call.
func TestGldbSessionDeleteFails(t *testing.T) {
	var failPrefix []byte // a Delete of a key with this prefix fails
	failures := 0
	st, gst, _ := faultStore(t, os.Stderr, func(op string, key []byte) error {
		if op == "Delete" && failPrefix != nil && bytes.HasPrefix(key, failPrefix) {
			failures++
			return errInjected
		}
		return nil
	})
	defer func() { st.PruneKill <- 0 }()

	deleted := st.NewSession([]byte("user"))
	consumed := st.NewSession([]byte("user"))
	tokens := make(map[*qsess.Session]string)
	for _, sess := range []*qsess.Session{deleted, consumed} {
		if err := sess.Save(nil); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
		token, _, err := sess.Token()
		if err != nil {
			t.Fatal("Token failed - " + err.Error())
		}
		tokens[sess] = token
	}

	failPrefix = gst.sessPrefix
	qstest.MustMatchClass(t, deleted.Delete(nil), qsess.ErrBackEnd, "Delete whose session record delete failed")
	_, err := st.ConsumeTokenSession(tokens[consumed])
	qstest.MustMatchClass(t, err, qsess.ErrBackEnd, "ConsumeTokenSession whose session record delete failed")
	if failures != 2 {
		t.Fatalf("Delete and ConsumeTokenSession deleted session records %d times, want 2", failures)
	}

	failPrefix = nil
	if _, _, err := st.GetTokenSession(tokens[deleted]); err != nil {
		t.Error("the session whose Delete failed is gone - " + err.Error())
	}
	if _, err := st.ConsumeTokenSession(tokens[consumed]); err != nil {
		t.Error("the token whose redemption failed cannot be redeemed - " + err.Error())
	}
	_, err = st.ConsumeTokenSession(tokens[consumed])
	qstest.MustMatchClass(t, err, qsess.ErrInvalidCredential, "a token redeemed a second time")
}

// TestGldbPrunerDeletesOrphans checks that a pass deletes, logging nothing, an expired expiration index record whose
// session record is gone, as a Delete whose delete of that index record fails leaves one, and so does a DeleteByUserID
// that cannot read the session record, and so cannot tell which index record names it.
func TestGldbPrunerDeletesOrphans(t *testing.T) {
	var failOp string
	var failKey []byte      // the call failOp fails once for this key
	var errLog bytes.Buffer // read only once the pruner has stopped
	st, gst, db := faultStore(t, &errLog, func(op string, key []byte) error {
		if op == failOp && failKey != nil && bytes.Equal(key, failKey) {
			failKey = nil
			return errInjected
		}
		return nil
	})

	var orphans [][]byte
	for _, call := range []string{"Delete", "DeleteByUserID"} {
		user := []byte("user of " + call)
		key := saveRecord(t, gst, user, -1)
		exp, err := gst.findExpiration(key)
		if err != nil {
			t.Fatal("findExpiration failed - " + err.Error())
		}
		eKey := gst.expKey(exp, key)
		if call == "Delete" {
			failOp = "Delete"
			failKey = eKey
			err = gst.Delete(key, nil)
		} else {
			failOp = "Get"
			failKey = key
			err = gst.DeleteByUserID(user)
		}
		if err != nil {
			t.Fatalf("%s failed - %v", call, err)
		}
		if failKey != nil {
			t.Fatalf("%s did not meet its failure", call)
		}
		if has, _ := db.Has(key, nil); has {
			t.Fatalf("%s left the session record", call)
		}
		orphans = append(orphans, eKey)
	}

	st.PruneInterval <- 3600 // a pass starts at once
	st.PruneKill <- 0        // taken once the pass is over
	for i, eKey := range orphans {
		if has, _ := db.Has(eKey, nil); has {
			t.Errorf("a pass kept expired expiration index record %d, whose session record is gone", i)
		}
	}
	if errLog.Len() != 0 {
		t.Errorf("the pass logged:\n%s", errLog.String())
	}
}

// TestGldbExpirySecond checks that Get, Save and Consume take a session as expired in the second its record names, as
// the map store does; a session saved for 0 seconds names the second it is saved in.
func TestGldbExpirySecond(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst := testStore.BackEnd().(*gldbStore)

	for range 20 {
		second := time.Now().Unix()
		keys := make([]gldbSessKey, 3)
		for i := range keys {
			keys[i] = saveRecord(t, gst, []byte("user"), 0)
		}
		_, _, _, _, _, errGet := gst.Get(keys[0], nil)
		errSave := gst.Save((*[]byte)(&keys[1]), []byte{2}, []byte("user"), 60, 0)
		_, _, _, _, _, errConsume := gst.Consume(keys[2], nil)
		if time.Now().Unix() != second {
			continue // the calls straddled a second, so they did not all fall in the expiry second
		}
		for call, err := range map[string]error{"Get": errGet, "Save": errSave, "Consume": errConsume} {
			if !errors.Is(err, qsess.ErrNotFound) {
				t.Errorf("%s in a session's expiry second: err = %v, want a miss", call, err)
			}
		}
		return
	}
	t.Fatal("each of twenty tries straddled a second")
}

// TestGldbDeleteByUserIDShortRecords checks that DeleteByUserID deletes a session record too short to hold an
// expiration time, with its user id index record, as revoking a session needs only its key.
func TestGldbDeleteByUserIDShortRecords(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	gst := testStore.BackEnd().(*gldbStore)

	for _, size := range []int{0, 3} {
		user := []byte(fmt.Sprintf("user %d", size))
		key := saveRecord(t, gst, user, 3600)
		if err := testGldb.Put(key, make([]byte, size), nil); err != nil {
			t.Fatal("Put failed - " + err.Error())
		}
		if err := gst.DeleteByUserID(user); err != nil {
			t.Errorf("DeleteByUserID over a %d-byte session record failed - %v", size, err)
		}
		for what, k := range map[string][]byte{"session record": key, "user id index record": gst.uidKey(user, key)} {
			if has, _ := testGldb.Has(k, nil); has {
				t.Errorf("DeleteByUserID over a %d-byte session record left its %s", size, what)
			}
		}
	}
}

// stressSession is a session that TestGldbStress saved, with what the calls on it have promised.
type stressSession struct {
	token    string
	user     string
	savedSeq int64        // taken once its first Save and its Token have returned
	revoked  atomic.Bool  // set once a Delete or ConsumeTokenSession of it has returned nil
	consumed atomic.Int32 // ConsumeTokenSession calls that returned it
}

// TestGldbStress runs every session operation of a goleveldb store at once, then checks what each revocation promised,
// and, after a pass of the pruner, that each session record left has its index records and no other record is left.
func TestGldbStress(t *testing.T) {
	var errLog bytes.Buffer // read only once the pruner has stopped
	st, gst, db := faultStore(t, &errLog, func(string, []byte) error { return nil })
	st.NewSessData = nil

	users := make([]string, 32)
	for i := range users {
		users[i] = fmt.Sprintf("user %d", i)
	}
	ops := []string{"login", "login", "login", "login", "refresh", "refresh", "refresh", "refresh", "read", "read",
		"read", "logout", "logout", "logout", "consume", "consume", "consume", "sign out everywhere", "replay",
		"replay"}
	lifetimes := []int{1, 3600, 3600}

	var seq atomic.Int64
	var mu sync.Mutex // guards sessions and signOuts
	var sessions []*stressSession
	signOuts := make(map[string]int64) // a user, and the latest seq at which a successful DeleteByUserID of it began

	login := func(user string, maxAgeSecs int) {
		sess := st.NewSession([]byte(user))
		sess.MaxAgeSecs = maxAgeSecs
		if err := sess.Save(nil); err != nil {
			t.Errorf("a first Save failed - %v", err)
			return
		}
		token, _, err := sess.Token()
		if err != nil {
			// a DeleteByUserID, or expiry, can come between the Save and Token
			if !errors.Is(err, qsess.ErrNotFound) {
				t.Errorf("Token after a first Save failed - %v", err)
			}
			return
		}
		mu.Lock()
		defer mu.Unlock()
		sessions = append(sessions, &stressSession{token: token, user: user, savedSeq: seq.Add(1)})
	}
	pick := func(r *rand.Rand) *stressSession {
		mu.Lock()
		defer mu.Unlock()
		recent := sessions[max(0, len(sessions)-64):] // an operation on a session that is gone exercises little
		return recent[r.Intn(len(recent))]
	}
	login(users[0], 3600) // so that pick has a session to pick
	if len(sessions) == 0 {
		t.FailNow()
	}

	deadline := time.Now().Add(2 * time.Second)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(g)))
			for time.Now().Before(deadline) {
				switch ops[r.Intn(len(ops))] {
				case "login":
					login(users[r.Intn(len(users))], lifetimes[r.Intn(len(lifetimes))])
				case "refresh":
					if sess, _, err := st.GetTokenSession(pick(r).token); err == nil {
						sess.MaxAgeSecs = lifetimes[r.Intn(len(lifetimes))]
						sess.Save(nil) // fails if the session has gone since
					}
				case "read":
					if sess, _, err := st.GetTokenSession(pick(r).token); err == nil {
						sess.Token()
					}
				case "logout":
					s := pick(r)
					sess, _, err := st.GetTokenSession(s.token)
					if err != nil {
						continue
					}
					if err := sess.Delete(nil); err != nil {
						t.Errorf("Delete failed - %v", err)
						continue
					}
					s.revoked.Store(true)
				case "consume":
					s := pick(r)
					if _, err := st.ConsumeTokenSession(s.token); err == nil {
						s.consumed.Add(1)
						s.revoked.Store(true)
					}
				case "sign out everywhere":
					user := users[r.Intn(len(users))]
					start := seq.Add(1)
					if err := st.NewSession([]byte(user)).DeleteByUserID(nil); err != nil {
						t.Errorf("DeleteByUserID failed - %v", err)
						continue
					}
					mu.Lock()
					signOuts[user] = max(signOuts[user], start)
					mu.Unlock()
				case "replay":
					s := pick(r)
					if !s.revoked.Load() {
						continue
					}
					if _, _, err := st.GetTokenSession(s.token); err == nil {
						t.Error("a session deleted or consumed earlier is live again")
					}
				}
			}
		}()
	}
	wg.Wait()

	time.Sleep(2100 * time.Millisecond) // until every session saved for a second has expired
	st.PruneInterval <- 3600            // the store's first pass, at once
	st.PruneKill <- 0                   // taken once it is over
	now := time.Now().Unix()

	revoked := 0
	for _, s := range sessions {
		if n := s.consumed.Load(); n > 1 {
			t.Errorf("a token was redeemed %d times", n)
		}
		_, _, err := st.GetTokenSession(s.token)
		if s.revoked.Load() {
			revoked++
			if err == nil {
				t.Error("a session deleted or consumed is live")
			}
		}
		if s.savedSeq < signOuts[s.user] && err == nil {
			t.Error("a session saved before a DeleteByUserID of its user began is live")
		}
	}

	sessRecords := make(map[string]gldbSessValue)
	indexRecords := make(map[string]bool)
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := string(iter.Key())
		switch {
		case strings.HasPrefix(key, string(gst.sessPrefix)):
			sessRecords[key] = gldbSessValue(bytes.Clone(iter.Value()))
		case strings.HasPrefix(key, string(gst.expPrefix)), strings.HasPrefix(key, string(gst.uidPrefix)):
			indexRecords[key] = true
		default:
			t.Errorf("a key outside the store's ranges: %x", key)
		}
	}
	iter.Release()
	for key, value := range sessRecords {
		if !value.valid() || value.expiration() < now {
			t.Errorf("session record %x is malformed or has expired, after the last pass", key)
			continue
		}
		for _, index := range []string{string(gst.expKey(value.expirationBytes(), []byte(key))),
			string(gst.uidKey(value.userID(), []byte(key)))} {
			if !indexRecords[index] {
				t.Errorf("session record %x lacks index record %x", key, index)
			}
			delete(indexRecords, index)
		}
	}
	for index := range indexRecords {
		t.Errorf("index record %x names no session record that it indexes", index)
	}
	if errLog.Len() != 0 {
		t.Errorf("the pruner logged:\n%s", errLog.String())
	}
	t.Logf("%d sessions saved, %d deleted or consumed, %d live at the end", len(sessions), revoked, len(sessRecords))
}
