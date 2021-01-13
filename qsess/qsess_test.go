// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests that DO see package internals

package qsess

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeTestStore(t *testing.T, delByUserID bool) *Store {
	var err error

	st, err := NewMapStore(
		[]byte("key-to-detect-tampering---------"),
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal("makeTestStore - NewMapStore failed - " + err.Error())
	}

	return st
}

// TestUidToClient tests that, when Store.uidToClient is set,
// user id's are properly sent to and received from the client.
func TestUidToClient(t *testing.T) {
	store := makeTestStore(t, false)
	store.uidToClient = true

	userid := []byte("userid-xyzzy")
	sess := store.NewSession(userid)

	w := httptest.NewRecorder()
	if err := sess.Save(w); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}

	// grab the newly-created cookie from w and make an r containing it.
	cookies, cookieOK := w.Header()["Set-Cookie"]
	if (!cookieOK) || len(cookies) != 1 {
		t.Fatal(errors.New("Set-Cookie header not present or wrong count"))
	}
	cookie := cookies[0]
	if cookie[:len(store.CookieName)+1] != (store.CookieName + "=") {
		t.Fatal(errors.New("wrong cookie name in response"))
	}
	cookieData := cookie[len(store.CookieName)+1 : strings.Index(cookie, ";")]
	r, _ := http.NewRequest("GET", "http://foo.com", nil)
	r.AddCookie(&http.Cookie{Name: store.CookieName, Value: cookieData})

	// wipe out my session's saved userID, then GetSession and check it

	sess.userID = []byte{}

	sess, _, err := store.GetSession(httptest.NewRecorder(), r)
	if err != nil {
		t.Fatal("GetSession failed - " + err.Error())
	}

	if bytes.Compare(userid, sess.userID) != 0 {
		t.Fatal("userid returned from client does not match original")
	}

	sess.Delete(w)
}
