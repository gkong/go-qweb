// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of what Session.Delete reports, which do NOT see package internals

package qsess_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

// failDeleteBackEnd is a back end whose Delete always fails, so that what Session.Delete reports
// when the database delete fails can be tested. Only Save, Get and Delete are reached.
type failDeleteBackEnd struct{}

func (b failDeleteBackEnd) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	*sessID = []byte{1}
	return nil
}

func (b failDeleteBackEnd) Get(sessID []byte, uID []byte) ([]byte, []byte, int, int, int, error) {
	return []byte{}, []byte{}, 60, 60, 30, nil
}

func (b failDeleteBackEnd) Delete(sessID []byte, uID []byte) error {
	return errors.New("database delete failed")
}

func (b failDeleteBackEnd) DeleteByUserID(userID []byte) error { return nil }

// failDeleteStore makes a store whose database delete always fails.
func failDeleteStore(t *testing.T) *qsess.Store {
	st, err := qsess.NewStore(failDeleteBackEnd{}, false, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal("NewStore failed - " + err.Error())
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

// TestDeleteReportsFailureWithNoDeleteToken checks that a token store with no DeleteToken
// function reports a failed database delete instead of returning nil while the session survives.
// There is no client-side delete to make up for it: the token stays in the client's hands and the
// record stays in the database.
// What it reports matters too: the absent DeleteToken is a supported arrangement, not the
// failure, so the message must name the database and not the missing function.
func TestDeleteReportsFailureWithNoDeleteToken(t *testing.T) {
	st := failDeleteStore(t)
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

// TestDeleteWithDeleteToken checks that a registered DeleteToken still absorbs a failed database
// delete, which is the documented contract: either delete succeeding is enough.
func TestDeleteWithDeleteToken(t *testing.T) {
	st := failDeleteStore(t)
	st.AuthType = qsess.TokenAuth
	called := false
	st.DeleteToken = func(w http.ResponseWriter) error {
		called = true
		return nil
	}

	if err := savedSession(t, st).Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed although DeleteToken succeeded - " + err.Error())
	}
	if !called {
		t.Fatal("DeleteToken was not called")
	}
}

// TestDeleteCookieStoreUnchanged checks that cookie stores are unaffected: deleting the cookie
// still absorbs a failed database delete.
func TestDeleteCookieStoreUnchanged(t *testing.T) {
	st := failDeleteStore(t)
	st.AuthType = qsess.CookieAuth

	if err := savedSession(t, st).Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed for a cookie store although the cookie was deleted - " + err.Error())
	}
}
