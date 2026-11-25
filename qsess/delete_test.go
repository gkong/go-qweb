// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of what Session.Delete reports, which do NOT see package internals

package qsess_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

var errDBDelete = errors.New("database delete failed")

var errAlreadyGone = fmt.Errorf("%w - record not there", qsess.ErrNotFound)

// failDeleteBackEnd is a back end whose Delete always fails with err, so that what Session.Delete
// reports when the database delete fails can be tested. Only Save, Get and Delete are reached.
type failDeleteBackEnd struct{ err error }

func (b failDeleteBackEnd) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	*sessID = []byte{1}
	return nil
}

func (b failDeleteBackEnd) Get(sessID []byte, uID []byte) ([]byte, []byte, int, int, int, error) {
	return []byte{}, []byte{}, 60, 60, 30, nil
}

func (b failDeleteBackEnd) Delete(sessID []byte, uID []byte) error {
	return b.err
}

func (b failDeleteBackEnd) DeleteByUserID(userID []byte) error { return nil }

// failDeleteStore makes a store whose database delete always fails with err.
func failDeleteStore(t *testing.T, err error) *qsess.Store {
	st, e := qsess.NewStore(failDeleteBackEnd{err}, false, []byte("key-for-encryption--------------"))
	if e != nil {
		t.Fatal("NewStore failed - " + e.Error())
	}
	st.NewSessData = nil // this back end keeps no session data

	return st
}

// savedSession returns a session saved in st.
func savedSession(t *testing.T, st *qsess.Store) *qsess.Session {
	sess := st.NewSession([]byte("user"))
	if err := sess.Save(httptest.NewRecorder()); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	return sess
}

// cookieDeleted reports whether w holds a Set-Cookie that deletes st's session cookie.
func cookieDeleted(w *httptest.ResponseRecorder, st *qsess.Store) bool {
	for _, c := range w.Result().Cookies() {
		if c.Name == st.CookieName && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

// TestDeleteReportsFailureWithNoDeleteToken checks that a token store with no DeleteToken
// function reports a failed database delete instead of returning nil while the session survives.
// What it reports matters too: the absent DeleteToken is a supported arrangement, not the
// failure, so the message must name the database and not the missing function.
func TestDeleteReportsFailureWithNoDeleteToken(t *testing.T) {
	st := failDeleteStore(t, errDBDelete)
	st.AuthType = qsess.TokenAuth

	err := savedSession(t, st).Delete(httptest.NewRecorder())
	if err == nil {
		t.Fatal("Delete returned nil although the database delete failed and nothing was deleted from the client")
	}
	if strings.Contains(err.Error(), "DeleteToken") {
		t.Fatal("Delete blamed the absent DeleteToken instead of the database - " + err.Error())
	}
	if !strings.Contains(err.Error(), "database") {
		t.Fatal("Delete did not report the database failure - " + err.Error())
	}
}

// TestDeleteSucceedsWithNoDeleteToken checks the ordinary case for the same kind of store, which
// is the arrangement Save documents as an alternative to SendToken - the application sends and
// discards tokens itself, so there is no DeleteToken. With the database delete succeeding, Delete
// returns nil.
func TestDeleteSucceedsWithNoDeleteToken(t *testing.T) {
	st := makeTestStore(t, false)
	st.AuthType = qsess.TokenAuth

	if err := savedSession(t, st).Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed for a token store with no DeleteToken - " + err.Error())
	}
}

// TestDeleteWithDeleteToken checks that a registered DeleteToken is still called when the database
// delete fails, and that its success does not hide the failure: the token it deleted is not the
// only copy there may be.
func TestDeleteWithDeleteToken(t *testing.T) {
	st := failDeleteStore(t, errDBDelete)
	st.AuthType = qsess.TokenAuth
	called := false
	st.DeleteToken = func(w http.ResponseWriter) error {
		called = true
		return nil
	}

	err := savedSession(t, st).Delete(httptest.NewRecorder())
	if !errors.Is(err, errDBDelete) {
		t.Fatalf("Delete did not report the failed database delete although DeleteToken succeeded - err = %v", err)
	}
	if !called {
		t.Fatal("DeleteToken was not called")
	}
}

// TestDeleteCookieStoreReportsFailure checks that deleting the cookie does not hide a failed
// database delete, because a copy of the cookie kept elsewhere is unaffected, and that the cookie is
// deleted all the same.
func TestDeleteCookieStoreReportsFailure(t *testing.T) {
	st := failDeleteStore(t, errDBDelete)
	st.AuthType = qsess.CookieAuth

	w := httptest.NewRecorder()
	err := savedSession(t, st).Delete(w)
	if !errors.Is(err, errDBDelete) {
		t.Fatalf("Delete did not report the failed database delete for a cookie store - err = %v", err)
	}
	if !cookieDeleted(w, st) {
		t.Fatal("Delete did not delete the cookie")
	}
}

// TestDeleteRecordAlreadyGone checks that a record already gone from the database, as a back end
// reports with ErrNotFound, is not a failure for any kind of store, and that the client's copy is
// deleted all the same.
func TestDeleteRecordAlreadyGone(t *testing.T) {
	st := failDeleteStore(t, errAlreadyGone)
	st.AuthType = qsess.CookieAuth
	w := httptest.NewRecorder()
	if err := savedSession(t, st).Delete(w); err != nil {
		t.Fatal("Delete failed for a cookie store whose record was already gone - " + err.Error())
	}
	if !cookieDeleted(w, st) {
		t.Fatal("Delete did not delete the cookie")
	}

	st.AuthType = qsess.TokenAuth
	if err := savedSession(t, st).Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed for a token store whose record was already gone - " + err.Error())
	}
	called := false
	st.DeleteToken = func(w http.ResponseWriter) error {
		called = true
		return nil
	}
	if err := savedSession(t, st).Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed for a DeleteToken store whose record was already gone - " + err.Error())
	}
	if !called {
		t.Fatal("DeleteToken was not called")
	}
}
