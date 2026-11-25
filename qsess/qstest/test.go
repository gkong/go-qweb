// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qstest contains shared test code for use by qsess back-ends.
package qstest

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
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

	// a second session expiring with the first, for consuming once it has expired
	_, consumer := st.BackEnd().(qsess.Consumer)
	var consumeTok string
	if consumer {
		consumeSess := st.NewSession([]byte("userid-foo"))
		consumeSess.MaxAgeSecs = 3
		consumeTok = saveToken(t, consumeSess)
	}

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
	MustMatchClass(t, getErr, qsess.ErrInvalidCredential, "Get of an expired session")

	if consumer {
		_, err := st.ConsumeTokenSession(consumeTok)
		MustMatchClass(t, err, qsess.ErrInvalidCredential, "ConsumeTokenSession of an expired session")
	}
}

// ErrorsTest checks that GetSession, GetTokenSession and ConsumeTokenSession report the right one of
// qsess's error classes, and only it, for a request that carries nothing, a value that does not decrypt
// and a session deleted, revoked or consumed, or a back end that cannot consume; that Token reports a
// deleted session the same way; and that Delete of a session already gone succeeds. The store must
// support DeleteByUserID; expired sessions are ExpirationTest's.
func ErrorsTest(t *testing.T, store *qsess.Store) {
	st := *store // make a copy, to mess with

	st.AuthType = qsess.CookieAuth
	r, _ := http.NewRequest("GET", "http://foo.com", nil)
	_, _, err := st.GetSession(httptest.NewRecorder(), r)
	MustMatchClass(t, err, qsess.ErrNoCredential, "no cookie")
	r.AddCookie(&http.Cookie{Name: st.CookieName, Value: ""})
	_, _, err = st.GetSession(httptest.NewRecorder(), r)
	MustMatchClass(t, err, qsess.ErrNoCredential, "empty cookie")

	st.AuthType = qsess.TokenAuth
	for _, header := range []string{"", "Basic dXNlcjpwYXNz", "Bearer "} {
		r, _ := http.NewRequest("GET", "http://foo.com", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		_, _, err := st.GetSession(httptest.NewRecorder(), r)
		MustMatchClass(t, err, qsess.ErrNoCredential, "Authorization header "+strconv.Quote(header))
	}

	// well-formed base64 that no key opens
	forged := base64.URLEncoding.EncodeToString(make([]byte, 40))
	r, _ = http.NewRequest("GET", "http://foo.com", nil)
	r.Header.Set("Authorization", "Bearer "+forged)
	_, _, err = st.GetSession(httptest.NewRecorder(), r)
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "token that does not decrypt")
	if errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("a token that does not decrypt was reported as a back-end miss - " + err.Error())
	}

	userID := []byte("userid-errors")

	sess := st.NewSession(userID)
	tok := saveToken(t, sess)
	if err := sess.Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed - " + err.Error())
	}
	_, _, err = st.GetTokenSession(tok)
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "token of a deleted session")
	if !errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("token of a deleted session: not reported as a back-end miss - " + err.Error())
	}
	_, _, err = sess.Token()
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "Token of a deleted session")
	if err := sess.Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete of a session deleted already failed - " + err.Error())
	}

	sess = st.NewSession(userID)
	tok = saveToken(t, sess)
	if err := sess.DeleteByUserID(httptest.NewRecorder()); err != nil {
		t.Fatal("DeleteByUserID failed - " + err.Error())
	}
	_, _, err = st.GetTokenSession(tok)
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "token of a revoked session")
	if err := sess.Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete of a revoked session failed - " + err.Error())
	}

	sess = st.NewSession(userID)
	tok = saveToken(t, sess)
	if _, ok := st.BackEnd().(qsess.Consumer); !ok {
		_, err = st.ConsumeTokenSession(tok)
		MustMatchClass(t, err, qsess.ErrApplication, "ConsumeTokenSession on a back end that cannot consume")
		if !strings.Contains(err.Error(), "not implemented") {
			t.Fatal("ConsumeTokenSession on a back end that cannot consume: not reported as unimplemented - " +
				err.Error())
		}
		sess.Delete(httptest.NewRecorder())
		return
	}
	_, err = st.ConsumeTokenSession(forged)
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "consuming a token that does not decrypt")
	if _, err := st.ConsumeTokenSession(tok); err != nil {
		t.Fatal("ConsumeTokenSession failed - " + err.Error())
	}
	_, err = st.ConsumeTokenSession(tok)
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "token consumed already")
	if !errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("token consumed already: not reported as a back-end miss - " + err.Error())
	}
}

// MustMatchClass fails the test unless err matches class, one of qsess.ErrNoCredential,
// ErrInvalidCredential, ErrBackEnd and ErrApplication, and none of the others. what describes err.
func MustMatchClass(t *testing.T, err error, class error, what string) {
	t.Helper()
	for _, c := range []error{qsess.ErrNoCredential, qsess.ErrInvalidCredential, qsess.ErrBackEnd,
		qsess.ErrApplication} {
		if errors.Is(err, c) != (c == class) {
			t.Fatalf("%s: err = %v, want an error that matches %q and no other class", what, err, class)
		}
	}
}

// BackEndFailureTest checks that GetTokenSession, ConsumeTokenSession, Save and Token report a back end
// that breakStore has made fail as ErrBackEnd, not as a miss, and that Delete reports it for a cookie store.
func BackEndFailureTest(t *testing.T, store *qsess.Store, breakStore func()) {
	st := *store // make a copy, to mess with
	st.AuthType = qsess.TokenAuth

	sess := st.NewSession([]byte("userid-failure"))
	tok := saveToken(t, sess)
	breakStore()

	_, _, err := st.GetTokenSession(tok)
	MustMatchClass(t, err, qsess.ErrBackEnd, "GetTokenSession on a failed back end")
	if errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("a failed back end was reported as a back-end miss - " + err.Error())
	}
	if _, ok := st.BackEnd().(qsess.Consumer); ok {
		_, err = st.ConsumeTokenSession(tok)
		MustMatchClass(t, err, qsess.ErrBackEnd, "ConsumeTokenSession on a failed back end")
		if errors.Is(err, qsess.ErrNotFound) {
			t.Fatal("a failed back end was reported as a back-end miss when consuming - " + err.Error())
		}
	}
	err = sess.Save(httptest.NewRecorder())
	MustMatchClass(t, err, qsess.ErrBackEnd, "Save on a failed back end")
	if errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("a failed back end was reported as a back-end miss when saving - " + err.Error())
	}
	_, _, err = sess.Token()
	MustMatchClass(t, err, qsess.ErrBackEnd, "Token on a failed back end")

	st.AuthType = qsess.CookieAuth
	err = sess.Delete(httptest.NewRecorder())
	MustMatchClass(t, err, qsess.ErrBackEnd, "Delete for a cookie store whose back end has failed")
}

// ExpiredSaveTest checks that a Save of a session that has expired, before anything has removed its record,
// fails as for a session that is gone rather than bringing the session back. Cassandra's Save, an upsert, does not.
func ExpiredSaveTest(t *testing.T, store *qsess.Store) {
	sess := store.NewSession([]byte("userid-expiredsave"))
	sess.MaxAgeSecs = 1
	if err := sess.Save(httptest.NewRecorder()); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	time.Sleep(2100 * time.Millisecond)

	sess.MaxAgeSecs = 3600
	err := sess.Save(httptest.NewRecorder())
	MustMatchClass(t, err, qsess.ErrInvalidCredential, "Save of an expired session")
	if !errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("Save of an expired session: not reported as a back-end miss - " + err.Error())
	}
}

// SaveAgainTest checks that a session saved again at once is saved, although the second Save may change
// nothing in its record.
func SaveAgainTest(t *testing.T, store *qsess.Store) {
	sess := store.NewSession([]byte("userid-saveagain"))
	for i := 0; i < 3; i++ {
		if err := sess.Save(httptest.NewRecorder()); err != nil {
			t.Fatalf("Save number %d in a row of one session failed - %s", i+1, err.Error())
		}
	}
	sess.Delete(httptest.NewRecorder())
}

func saveToken(t *testing.T, sess *qsess.Session) string {
	if err := sess.Save(httptest.NewRecorder()); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	tok, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}
	return tok
}

// ConsumeTest is a back-end-independent test of Store.ConsumeTokenSession, for a back end that
// implements qsess.Consumer. It checks that the first call returns the session, that a second
// call and an ordinary Get both fail afterwards, and that of many concurrent calls for one
// token exactly one succeeds.
func ConsumeTest(t *testing.T, store *qsess.Store) {
	st := *store // make a copy, to mess with
	st.AuthType = qsess.TokenAuth

	msg := "single use only"
	userid := []byte("userid-consume")

	sess := st.NewSession(userid)
	sess.MaxAgeSecs = 60
	sess.Data.(*qsess.VarMap).Vars["note"] = msg
	if err := sess.Save(httptest.NewRecorder()); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	tok, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}

	consumed, err := st.ConsumeTokenSession(tok)
	if err != nil {
		t.Fatal("ConsumeTokenSession failed - " + err.Error())
	}
	if bytes.Compare(consumed.UserID(), userid) != 0 {
		t.Fatal("ConsumeTokenSession did not return the session's user id")
	}
	if consumed.Data.(*qsess.VarMap).Vars["note"].(string) != msg {
		t.Fatal("ConsumeTokenSession did not return the session's data")
	}
	if _, err := st.ConsumeTokenSession(tok); err == nil {
		t.Fatal("ConsumeTokenSession succeeded twice for one token")
	}
	if _, _, err := st.GetTokenSession(tok); err == nil {
		t.Fatal("GetTokenSession succeeded after the token was consumed")
	}

	// many presentations of one token at once: exactly one must be redeemed
	const racers = 8

	sess = st.NewSession(userid)
	sess.MaxAgeSecs = 60
	if err := sess.Save(httptest.NewRecorder()); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	if tok, _, err = sess.Token(); err != nil {
		t.Fatal("Token failed - " + err.Error())
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := st.ConsumeTokenSession(tok)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	redeemed := 0
	for err := range results {
		if err == nil {
			redeemed++
		}
	}
	if redeemed != 1 {
		t.Fatalf("%d of %d concurrent ConsumeTokenSession calls succeeded; want exactly 1", redeemed, racers)
	}
}

// DeleteByUserIdTest is a backend-independent test of DeleteByUserID.
//
// noZombies - require a Save of a revoked session to fail with ErrInvalidCredential and ErrNotFound
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
		MustMatchClass(t, err, qsess.ErrInvalidCredential, "Save of a revoked session")
		if !errors.Is(err, qsess.ErrNotFound) {
			t.Fatal("Save of a revoked session: not reported as a back-end miss - " + err.Error())
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
