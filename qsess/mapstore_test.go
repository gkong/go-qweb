// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests that do NOT see package internals

package qsess_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

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

// TestMapConcurrentExpiredGet presents many expired sessions at once. Get deletes a session it
// finds expired, which it must not do under the lock that concurrent readers share.
func TestMapConcurrentExpiredGet(t *testing.T) {
	const sessions = 4000

	st := makeTestStore(t, false)
	st.AuthType = qsess.TokenAuth
	tokens := make([]string, 0, sessions)
	st.SendToken = func(token string, timeToLiveSecs int, w http.ResponseWriter) error {
		tokens = append(tokens, token)
		return nil
	}

	for i := 0; i < sessions; i++ {
		sess := st.NewSession([]byte("user"))
		sess.MaxAgeSecs = 1
		if err := sess.Save(nil); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
	}

	time.Sleep(1100 * time.Millisecond)

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, sessions)
	for _, token := range tokens {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			_, _, err := st.GetTokenSession(token)
			results <- err
		}(token)
	}
	close(start)
	wg.Wait()
	close(results)

	for err := range results {
		if err == nil {
			t.Fatal("GetTokenSession succeeded for an expired session")
		}
	}
}
