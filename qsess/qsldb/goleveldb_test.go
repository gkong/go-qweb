// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsldb

import (
	"bytes"
	"flag"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
	"github.com/syndtr/goleveldb/leveldb"
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

	before := countKeysWithPrefix(foreignPrefix)
	if before == 0 {
		t.Fatal("the second store wrote no keys")
	}

	storeA.PruneInterval <- 1
	time.Sleep(3 * time.Second)

	if after := countKeysWithPrefix(foreignPrefix); after != before {
		t.Errorf("storeA's pruner deleted the second store's records - %d keys before, %d after",
			before, after)
	}
	if _, _, _, _, _, err := gstB.Get(keyB, nil); err != nil {
		t.Errorf("the second store can no longer read its own session - %v", err)
	}
}

func countKeysWithPrefix(prefix []byte) int {
	n := 0
	iter := testGldb.NewIterator(nil, nil)
	for ok := iter.Seek(prefix); ok; ok = iter.Next() {
		if !bytes.HasPrefix(iter.Key(), prefix) {
			break
		}
		n++
	}
	iter.Release()
	return n
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

// TestGldbBinding checks that NewGldbStore names a store after its prefix and that two stores
// sharing one database and one key list keep their values apart. Nothing has to collide for this
// to matter: a session key carries the prefix of the store that issued it and Get uses the key
// verbatim, so without Name each store reads the other's records. Tokens only: the cookie path
// never reaches the back end and is covered by TestBindingCookies in qsess.
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

	// a value issued before Name existed carries no binding and is accepted by default by
	// whichever store receives it, until AcceptUnbound is cleared. Accepting it here means
	// reading, through b, the record a wrote under a's own prefix.
	a.Name = ""
	tokenUnbound := gldbSaveToken(t, a)
	if _, _, err := b.GetTokenSession(tokenUnbound); err != nil {
		t.Fatal("unbound token rejected by default - " + err.Error())
	}
	b.AcceptUnbound = false
	if _, _, err := b.GetTokenSession(tokenUnbound); err == nil || !strings.Contains(err.Error(), "decode") {
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
