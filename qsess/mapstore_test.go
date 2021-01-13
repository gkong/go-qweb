// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests that do NOT see package internals

package qsess_test

import (
	"testing"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
)

func makeTestStore(t *testing.T, delByUserId bool) *qsess.Store {
	var err error

	st, err := qsess.NewMapStore(
		[]byte("key-to-detect-tampering---------"),
		[]byte("key-for-encryption--------------"),
	)
	if err != nil {
		t.Fatal("makeTestStore - NewMapStore failed - " + err.Error())
	}

	return st
}

func TestMapSanity(t *testing.T) {
	st := makeTestStore(t, false)
	qstest.SanityTest(t, st)
}

func TestMapNoSessData(t *testing.T) {
	st := makeTestStore(t, false)
	qstest.NoSessDataTest(t, st)
}

func TestMapDeleteByUserId(t *testing.T) {
	st := makeTestStore(t, true)
	qstest.DeleteByUserIDTest(t, st, true)
}

func TestMapExpiration(t *testing.T) {
	st := makeTestStore(t, false)
	qstest.ExpirationTest(t, st)
}
