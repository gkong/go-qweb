// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of Store.ConsumeTokenSession, which do NOT see package internals

package qsess_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
)

// plainBackEnd implements SessBackEnd and nothing more, so a store built on it cannot consume
// atomically. Just enough of it works for ConsumeTokenSession to reach its Consumer check.
type plainBackEnd struct{}

func (b plainBackEnd) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	*sessID = []byte{1}
	return nil
}

func (b plainBackEnd) Get(sessID []byte, uID []byte) ([]byte, []byte, int, int, int, error) {
	return []byte{}, []byte{}, 60, 60, 30, nil
}

func (b plainBackEnd) Delete(sessID []byte, uID []byte) error { return nil }

func (b plainBackEnd) DeleteByUserID(userID []byte) error { return nil }

// TestMapConsume runs the shared ConsumeTokenSession test against the in-memory back end.
func TestMapConsume(t *testing.T) {
	qstest.ConsumeTest(t, makeTestStore(t, false))
}

// TestConsumeNeedsConsumer checks that ConsumeTokenSession fails on a back end that does not
// implement Consumer, rather than consuming non-atomically. The ordinary Get at the end proves
// the store works otherwise, so the failure is the Consumer check and nothing else.
func TestConsumeNeedsConsumer(t *testing.T) {
	st, err := qsess.NewStore(plainBackEnd{}, false, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal("NewStore failed - " + err.Error())
	}
	st.AuthType = qsess.TokenAuth
	st.NewSessData = nil // this back end keeps no session data

	sess := st.NewSession([]byte("user"))
	if err := sess.Save(httptest.NewRecorder()); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	tok, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}

	_, err = st.ConsumeTokenSession(tok)
	if err == nil {
		t.Fatal("ConsumeTokenSession succeeded on a back end that does not implement Consumer")
	}
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "ConsumeTokenSession on a back end that is not a Consumer")
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatal("ConsumeTokenSession on a back end that is not a Consumer: not reported as unimplemented - " +
			err.Error())
	}
	if _, _, err := st.GetTokenSession(tok); err != nil {
		t.Fatal("GetTokenSession failed, so the test proves nothing about Consumer - " + err.Error())
	}
}
