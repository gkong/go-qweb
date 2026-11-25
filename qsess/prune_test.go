// Copyright 2026 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of the map store's expiry checks, pruner and user id index, which DO see package internals

package qsess

import (
	"errors"
	"maps"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
)

// mapCounts returns the number of sessions in ms, and how many its index holds for each user id.
func mapCounts(ms *mapStore) (int, map[string]int) {
	ms.RLock()
	defer ms.RUnlock()

	users := make(map[string]int)
	for userID, ids := range ms.uindex {
		users[userID] = len(ids)
	}
	return len(ms.sess), users
}

// pruneTestStore makes a map store in the current bubble, so that its pruner runs on the bubble's clock,
// and returns it with functions that save a session in it and check what it holds.
func pruneTestStore(t *testing.T) (*Store, func(*Session, int), func(string, int, map[string]int)) {
	st, err := NewMapStore([]byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal("NewMapStore failed - " + err.Error())
	}
	ms := st.backEnd.(*mapStore)

	save := func(sess *Session, maxAgeSecs int) {
		sess.MaxAgeSecs = maxAgeSecs
		if err := sess.Save(httptest.NewRecorder()); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
	}
	check := func(when string, wantSessions int, wantUsers map[string]int) {
		t.Helper()
		sessions, users := mapCounts(ms)
		if sessions != wantSessions || !maps.Equal(users, wantUsers) {
			t.Fatalf("%s: %d sessions, index %v; want %d, index %v", when, sessions, users, wantSessions, wantUsers)
		}
	}
	return st, save, check
}

// TestMapPrune checks that the pruner deletes the sessions expired at each pass, with the index entries of
// user ids left with none, and spares a session refreshed before it expired; and that a new interval from
// PruneInterval brings a pass at once and takes effect. In the bubble, its minutes pass instantly.
func TestMapPrune(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, save, check := pruneTestStore(t)
		// Test cannot return while the pruner runs
		defer func() { st.PruneKill <- 0 }()

		start := time.Now()
		pass := mapPruneIntervalSecs * time.Second

		save(st.NewSession([]byte("gone")), 60)
		save(st.NewSession([]byte("gone")), 60)
		save(st.NewSession([]byte("kept")), 60)
		save(st.NewSession([]byte("kept")), 3600)
		refreshed := st.NewSession([]byte("refreshed"))
		save(refreshed, 110) // would expire before the first pass

		time.Sleep(time.Until(start.Add(100 * time.Second)))
		save(refreshed, 60) // now it expires 40 seconds after the first pass

		time.Sleep(time.Until(start.Add(pass + time.Second)))
		check("after the first pass", 2, map[string]int{"kept": 1, "refreshed": 1})

		time.Sleep(time.Until(start.Add(2*pass + time.Second)))
		check("after the second pass", 1, map[string]int{"kept": 1})

		save(st.NewSession([]byte("stale")), 1)
		time.Sleep(time.Second) // stale has expired, and no pass is due
		save(st.NewSession([]byte("brief")), 1)
		st.PruneInterval <- 1
		synctest.Wait()
		check("at once on a new interval", 2, map[string]int{"kept": 1, "brief": 1})
		time.Sleep(1500 * time.Millisecond)
		check("a second after a new interval of one second", 1, map[string]int{"kept": 1})
	})
}

// TestMapPruneLargeStore checks that a pass over a store holding many batches of sessions, during which the
// pruner lets go of the lock between batches, deletes every expired session and nothing else.
func TestMapPruneLargeStore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, save, check := pruneTestStore(t)
		defer func() { st.PruneKill <- 0 }()

		const sessions = 5*mapPruneBatch + 7
		for i := 0; i < sessions; i++ {
			if i%3 == 0 {
				save(st.NewSession([]byte("kept")), 3600)
			} else {
				save(st.NewSession([]byte("gone")), 60)
			}
		}

		time.Sleep(mapPruneIntervalSecs*time.Second + time.Second)
		kept := (sessions + 2) / 3
		check("after a pass", kept, map[string]int{"kept": kept})
	})
}

// TestMapDeleteExpiredRechecks checks that deleteExpired, which Get calls on finding a session expired, deletes
// the session only if it is still expired when deleteExpired holds the write lock.
func TestMapDeleteExpiredRechecks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, save, check := pruneTestStore(t)
		defer func() { st.PruneKill <- 0 }()
		ms := st.backEnd.(*mapStore)

		live := st.NewSession([]byte("live"))
		save(live, 60)
		expired := st.NewSession([]byte("expired"))
		save(expired, 1)
		time.Sleep(2 * time.Second)

		ms.deleteExpired(bytesToID(live.sessID))
		ms.deleteExpired(bytesToID(expired.sessID))
		check("after deleteExpired of a live session and an expired one", 1, map[string]int{"live": 1})
	})
}

// TestMapExpirySecond checks that a Save or Consume in the very second a session expires fails, as Get then reports
// a session that expires with it expired; no time passes in the bubble while the three are saved.
func TestMapExpirySecond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, save, _ := pruneTestStore(t)
		defer func() { st.PruneKill <- 0 }()

		start := time.Now()
		saved := st.NewSession([]byte("user"))
		save(saved, 2)
		other := st.NewSession([]byte("user"))
		save(other, 2)
		consumed := st.NewSession([]byte("user"))
		save(consumed, 2)

		time.Sleep(time.Until(start.Add(2 * time.Second)))
		// a Get of the saved session would delete it, and so hide the boundary
		if _, _, _, _, _, err := st.backEnd.Get(other.sessID, nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get at expiry: err = %v, want a miss", err)
		}
		if _, _, _, _, _, err := st.backEnd.(*mapStore).Consume(consumed.sessID, nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Consume at expiry: err = %v, want a miss", err)
		}
		saved.MaxAgeSecs = 60
		if err := saved.Save(httptest.NewRecorder()); !errors.Is(err, ErrInvalidCredential) {
			t.Fatalf("Save at expiry: err = %v, want ErrInvalidCredential", err)
		}
	})
}

// TestMapIndexShrinks checks that a user id leaves the index with its last session, whether that is
// deleted, revoked or consumed.
func TestMapIndexShrinks(t *testing.T) {
	st := makeTestStore(t, false)
	st.AuthType = TokenAuth
	ms := st.backEnd.(*mapStore)

	deleted := st.NewSession([]byte("deleted"))
	revoked := st.NewSession([]byte("revoked"))
	consumed := st.NewSession([]byte("consumed"))
	for _, sess := range []*Session{deleted, revoked, consumed} {
		if err := sess.Save(nil); err != nil {
			t.Fatal("Save failed - " + err.Error())
		}
	}
	token, _, err := consumed.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}

	if err := deleted.Delete(nil); err != nil {
		t.Fatal("Delete failed - " + err.Error())
	}
	if err := revoked.DeleteByUserID(nil); err != nil {
		t.Fatal("DeleteByUserID failed - " + err.Error())
	}
	if _, err := st.ConsumeTokenSession(token); err != nil {
		t.Fatal("ConsumeTokenSession failed - " + err.Error())
	}

	if sessions, users := mapCounts(ms); sessions != 0 || len(users) != 0 {
		t.Fatalf("%d sessions and index %v left; want none", sessions, users)
	}
}
