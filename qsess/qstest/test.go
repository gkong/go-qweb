// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qstest contains shared test code for use by qsess back-ends.
package qstest

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gkong/go-qweb/qsess"
)

func TestNothing(t *testing.T) {
	return
}

func roundtrip(t *testing.T, sess *qsess.Session, store *qsess.Store) (*qsess.Session, *httptest.ResponseRecorder, *http.Request) {
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

	newsess, _, err := store.GetSession(httptest.NewRecorder(), r)
	if err != nil {
		t.Fatal("GetSession failed - " + err.Error())
	}

	return newsess, w, r
}

// SanityTest is a back-end-independent, minimal sanity test.
// It makes a session, saves it, reads it back, and checks retrieved
// session data against the original.
func SanityTest(t *testing.T, store *qsess.Store) {
	msg := "Hello, World!"
	userid := []byte("xyzzy")

	s1 := store.NewSession(userid)

	s1.MaxAgeSecs = 10
	s1.MinRefreshSecs = 5

	sdata := s1.Data.(*qsess.VarMap)
	sdata.Vars["note"] = msg

	s2, w, _ := roundtrip(t, s1, store)

	if bytes.Compare(s2.UserID(), userid) != 0 {
		t.Fatal("failed to persist userID")
	}

	if s2.MaxAgeSecs != 10 || s2.MinRefreshSecs != 5 {
		t.Fatal("failed to persist MaxAgeSecs/MinRefreshSecs")
	}

	sdata = s2.Data.(*qsess.VarMap)
	if sdata.Vars["note"].(string) != msg {
		t.Fatal(errors.New("retrieved session data does not match saved session data"))
	}

	s2.Delete(w)
}

// ExpirationTest is a backend-independent session expiration test.
func ExpirationTest(t *testing.T, store *qsess.Store) {
	st := *store // make a copy, to mess with
	st.AuthType = qsess.TokenAuth

	st.MaxAgeSecs = 33     // large number, to verify overridden by value in Session
	st.MinRefreshSecs = 55 // don't refresh

	sess := st.NewSession([]byte("userid-foo"))

	sess.MaxAgeSecs = 3
	sess.MinRefreshSecs = 5
	err := sess.Save(httptest.NewRecorder())
	if err != nil {
		t.Fatalf("Save failed - %s", err.Error())
	}

	tok, _, err := sess.Token()
	r, _ := http.NewRequest("GET", "http://nowhere.com", nil)
	r.Header.Add("Authorization", "Bearer "+tok)

	sess, _, getErr := st.GetSession(httptest.NewRecorder(), r)
	if getErr != nil {
		t.Fatalf("first Get failed, but session should not have expired yet - %s", getErr.Error())
	}

	time.Sleep(time.Second)

	sess, _, getErr = st.GetSession(httptest.NewRecorder(), r)
	if getErr != nil {
		t.Fatalf("second Get failed, but session should not have expired yet - %s", getErr.Error())
	}

	time.Sleep(4 * time.Second)

	sess, _, getErr = st.GetSession(httptest.NewRecorder(), r)
	if getErr == nil {
		t.Fatal("Get succeeded, but session should be expired")
	}
}

// DeleteByUserIdTest is a backend-independent test of DeleteByUserID.
//
// noZombies - report an error if Saving a deleted session succeeds
func DeleteByUserIDTest(t *testing.T, store *qsess.Store, noZombies bool) {
	var sessions = []struct {
		userid string
		msg    string
		sess   *qsess.Session
		w      *httptest.ResponseRecorder
		r      *http.Request
	}{
		{userid: "user1", msg: "msg1"},
		{userid: "user2", msg: "msg2"},
		{userid: "user1", msg: "msg3"},
		{userid: "user3", msg: "msg4"},
		{userid: "user1", msg: "msg5"},
	}

	for i := range sessions {
		// make and save session
		s := &sessions[i]
		s.sess = store.NewSession([]byte(s.userid))
		sd := s.sess.Data.(*qsess.VarMap)
		sd.Vars["note"] = s.msg
		s.w = httptest.NewRecorder()
		if err := s.sess.Save(s.w); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}

		// prepare to read it back
		cookies, cookieOK := s.w.Header()["Set-Cookie"]
		if (!cookieOK) || len(cookies) != 1 {
			t.Fatal(errors.New("Set-Cookie header not present or wrong count"))
		}
		cookie := cookies[0]
		if cookie[:len(store.CookieName)+1] != (store.CookieName + "=") {
			t.Fatal(errors.New("wrong cookie name in response"))
		}
		cookieData := cookie[len(store.CookieName)+1 : strings.Index(cookie, ";")]
		s.r, _ = http.NewRequest("GET", "http://foo.com", nil)
		s.r.AddCookie(&http.Cookie{Name: store.CookieName, Value: cookieData})
	}

	// issue one DeleteByUserId, which should delete the 3 sessions for user1
	if err := sessions[0].sess.DeleteByUserID(sessions[0].w); err != nil {
		t.Fatal("DeleteByUserID failed - " + err.Error())
	}

	// now check which sessions remain and which have disappeared
	for i := range sessions {
		s := &sessions[i]
		sess, _, err := store.GetSession(httptest.NewRecorder(), s.r)
		if s.userid == "user1" {
			if err == nil {
				t.Fatalf("session %d (msg = %s) should be gone but still exists", i, sess.Data.(*qsess.VarMap).Vars["note"].(string))
			}
		} else {
			if err != nil {
				t.Fatalf("session %d unexpectedly deleted - %s", i, err.Error())
			}
		}
	}

	if noZombies {
		// try to Save a session that has been deleted and
		// verify that Save fails and subsequent Get also fails

		s := &sessions[0]

		// prepare to Get
		cookie := s.w.Header()["Set-Cookie"][0]
		cookieData := cookie[len(store.CookieName)+1 : strings.Index(cookie, ";")]
		s.r, _ = http.NewRequest("GET", "http://foo.com", nil)
		s.r.AddCookie(&http.Cookie{Name: store.CookieName, Value: cookieData})

		// attempt to Save (should fail)
		s.w = httptest.NewRecorder()
		err := s.sess.Save(s.w)
		if err == nil {
			t.Errorf("Saving a deleted session should fail, but it succeeded.")
		}

		// attempt to Get (should fail)
		_, _, err = store.GetSession(httptest.NewRecorder(), s.r)
		if err == nil {
			t.Fatalf("DeleteByUserId - Save - Get should fail, but it succeeded")
		}
	}
}

// NoSessDataTest is a backend-independent test of noSessData
// (i.e. that back-ends don't blow up when given zero-length data).
func NoSessDataTest(t *testing.T, store *qsess.Store) {
	storecopy := *store
	st := &storecopy

	st.NewSessData = nil

	userid := []byte("xyzzy")

	s1 := st.NewSession(userid)

	s2, w, _ := roundtrip(t, s1, st)

	if bytes.Compare(s2.UserID(), userid) != 0 {
		t.Fatal("failed to persist userID")
	}

	s2.Delete(w)
}
