// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of sessions that were never saved, which do NOT see package internals

package qsess_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

// TestDeleteUnsavedSession checks that Delete, given a session with no id yet, leaves the back end
// alone and still deletes the client's copy.
func TestDeleteUnsavedSession(t *testing.T) {
	st := makeTestStore(t, false)

	w := httptest.NewRecorder()
	if err := st.NewSession([]byte("user")).Delete(w); err != nil {
		t.Fatal("Delete failed for a cookie store - " + err.Error())
	}
	if len(w.Header()["Set-Cookie"]) != 1 {
		t.Fatal("Delete did not delete the cookie")
	}

	st.AuthType = qsess.TokenAuth
	if err := st.NewSession([]byte("user")).Delete(httptest.NewRecorder()); err != nil {
		t.Fatal("Delete failed for a token store - " + err.Error())
	}
}

// TestTokenUnsavedSession checks that Token reports an error for a session with no id yet.
func TestTokenUnsavedSession(t *testing.T) {
	st := makeTestStore(t, false)
	st.AuthType = qsess.TokenAuth

	if _, _, err := st.NewSession([]byte("user")).Token(); err == nil {
		t.Fatal("Token succeeded for a session that was never saved")
	}
}
