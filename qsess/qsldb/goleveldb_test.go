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

func TestGldbExpiration(t *testing.T) {
	testStore := gldbTestStore(t)
	defer func() { testStore.PruneKill <- 0 }()
	qstest.ExpirationTest(t, testStore)
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

	// put a mal-formed record into the session expiration index, for pruner
	badKey := bscat(gst.expPrefix, []byte{4, 5, 6})
	testGldb.Put(badKey, []byte{}, nil)
	if _, err := gst.db.Get(badKey, nil); err != nil {
		t.Error("bad key not inserted")
	}

	// expireTest with pruner
	expireTest(t, testStore, gst, true)

	if _, err := testGldb.Get(badKey, nil); err == nil {
		t.Error("bad key still there; should have been pruned")
	}
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
