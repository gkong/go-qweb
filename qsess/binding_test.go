// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of Store.Name, Store.Binding and Store.AcceptUnbound, which do NOT see package internals

package qsess_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

var (
	bindingKey1 = []byte("binding-test-key-1--------------")
	bindingKey2 = []byte("binding-test-key-2--------------")
)

// tokenStore makes a token-auth map store with the given keys and the Name its constructor gave it.
func tokenStore(t *testing.T, keys ...[]byte) *qsess.Store {
	st, err := qsess.NewMapStore(keys...)
	if err != nil {
		t.Fatal("NewMapStore failed - " + err.Error())
	}
	st.AuthType = qsess.TokenAuth
	return st
}

// bindingStore makes a token-auth map store with the given keys and Name ("" stands in for a
// store of the library before Name existed) and saves one session in it, so that every store in
// a test holds a session with the same back-end id (the map store numbers sessions from 1000)
// and a value from one store refers to a live session in every other. It returns the store and
// that session's token.
func bindingStore(t *testing.T, name string, keys ...[]byte) (*qsess.Store, string) {
	st := tokenStore(t, keys...)
	st.Name = name
	return st, saveToken(t, st)
}

// saveToken saves a new session in st and returns its token.
func saveToken(t *testing.T, st *qsess.Store) string {
	sess := st.NewSession([]byte("user"))
	if err := sess.Save(nil); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	token, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}
	return token
}

// saveCookie saves a new session in the cookie store st and returns a request carrying the
// resulting cookie.
func saveCookie(t *testing.T, st *qsess.Store) *http.Request {
	w := httptest.NewRecorder()
	if err := st.NewSession([]byte("user")).Save(w); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	for _, c := range w.Result().Cookies() {
		if c.Name == st.CookieName {
			r.AddCookie(c)
			return r
		}
	}
	t.Fatal("Save set no session cookie")
	return nil
}

// rejected reports whether err is a failure to decode a value, as opposed to success or a
// back-end miss, so that a test proves a value was refused before the back end was consulted
// and not merely not found.
func rejected(err error) bool {
	return err != nil && strings.Contains(err.Error(), "decode")
}

// mustRejectToken fails the test unless st rejects token. what describes the token.
func mustRejectToken(t *testing.T, st *qsess.Store, token string, what string) {
	t.Helper()
	if _, _, err := st.GetTokenSession(token); !rejected(err) {
		t.Fatalf("%s: not rejected, err = %v", what, err)
	}
}

// mustAcceptToken fails the test unless st accepts token. what describes the token.
func mustAcceptToken(t *testing.T, st *qsess.Store, token string, what string) {
	t.Helper()
	if _, _, err := st.GetTokenSession(token); err != nil {
		t.Fatalf("%s: rejected - %v", what, err)
	}
}

// TestBindingSeparatesStores checks that a store accepts only values bound to it: map stores as
// constructed, with nothing set, and stores given a Name. Stores whose Name is cleared, as every
// store was before Name existed, still accept each other's values, and AcceptUnbound, set by
// default, makes no difference to them.
func TestBindingSeparatesStores(t *testing.T) {
	d1 := tokenStore(t, bindingKey1)
	d2 := tokenStore(t, bindingKey1)
	if d1.Name == "" || d2.Name == "" || d1.Name == d2.Name {
		t.Fatalf("NewMapStore did not give each store its own Name: %q, %q", d1.Name, d2.Name)
	}
	tokenD1 := saveToken(t, d1)
	saveToken(t, d2)
	mustAcceptToken(t, d1, tokenD1, "own token, map store as constructed")
	mustRejectToken(t, d2, tokenD1, "token from one map store as constructed presented to another")

	a, tokenA := bindingStore(t, "a", bindingKey1)
	b, tokenB := bindingStore(t, "b", bindingKey1)
	mustAcceptToken(t, a, tokenA, "own token")
	mustRejectToken(t, b, tokenA, "token bound to a presented to b")
	mustRejectToken(t, a, tokenB, "token bound to b presented to a")

	_, tokenU := bindingStore(t, "", bindingKey1)
	u, _ := bindingStore(t, "", bindingKey1)
	if !u.AcceptUnbound || !qsess.DefaultAcceptUnbound {
		t.Fatal("AcceptUnbound is not set by default")
	}
	mustAcceptToken(t, u, tokenU, "token from an unbound store presented to another unbound store")
	u.AcceptUnbound = false
	mustAcceptToken(t, u, tokenU, "the same, with AcceptUnbound cleared")
}

// TestBindingScope checks that Binding narrows a Name: stores with the same Name accept each
// other's values only with the same Binding, and a Name is never mistaken for a Binding.
func TestBindingScope(t *testing.T) {
	a1, tokenA1 := bindingStore(t, "a", bindingKey1)
	a2, _ := bindingStore(t, "a", bindingKey1)
	mustAcceptToken(t, a2, tokenA1, "token from a store with the same Name and no Binding")

	a1.Binding = "copy-1"
	a2.Binding = "copy-2"
	tokenCopy1 := saveToken(t, a1)
	saveToken(t, a2)
	mustAcceptToken(t, a1, tokenCopy1, "own token with Binding")
	mustRejectToken(t, a2, tokenCopy1, "token bound to copy-1 presented to copy-2")
	a2.Binding = "copy-1"
	mustAcceptToken(t, a2, tokenCopy1, "token from a store with the same Name and Binding")

	x, tokenX := bindingStore(t, "x", bindingKey1)
	y, _ := bindingStore(t, "", bindingKey1)
	y.Binding = "x"
	tokenY := saveToken(t, y)
	mustRejectToken(t, y, tokenX, "token bound to Name x presented to a store with Binding x")
	mustRejectToken(t, x, tokenY, "token bound to Binding x presented to a store with Name x")
}

// TestBindingCookies checks the cookie path: a cookie set by one store is refused by another
// store with the same keys and a different Name.
func TestBindingCookies(t *testing.T) {
	a, _ := bindingStore(t, "a", bindingKey1)
	b, _ := bindingStore(t, "b", bindingKey1)
	a.AuthType = qsess.CookieAuth
	b.AuthType = qsess.CookieAuth

	r := saveCookie(t, a)
	saveCookie(t, b)

	if _, _, err := a.GetSession(httptest.NewRecorder(), r); err != nil {
		t.Fatal("own cookie rejected - " + err.Error())
	}
	if _, _, err := b.GetSession(httptest.NewRecorder(), r); !rejected(err) {
		t.Fatalf("cookie bound to a presented to b: not rejected, err = %v", err)
	}
}

// TestBindingTransition checks the upgrade: a store accepts, by default, values issued before
// Name existed, under any of its keys, and nothing bound to another store; values it issues
// meanwhile are bound; and clearing AcceptUnbound ends the acceptance of the old values. Store b
// has rotated to a second key, so the old token, sealed under the first key, is accepted only if
// the unbound attempt is made for every key.
func TestBindingTransition(t *testing.T) {
	_, tokenOld := bindingStore(t, "", bindingKey1) // unbound, as every value was before Name
	_, tokenA := bindingStore(t, "a", bindingKey1)
	b, tokenB := bindingStore(t, "b", bindingKey2, bindingKey1)

	if !b.AcceptUnbound {
		t.Fatal("AcceptUnbound is not set by default")
	}
	mustAcceptToken(t, b, tokenOld, "unbound token, AcceptUnbound by default")
	mustAcceptToken(t, b, tokenB, "own token, AcceptUnbound by default")
	mustRejectToken(t, b, tokenA, "token bound to a, AcceptUnbound by default")

	tokenNew := saveToken(t, b)
	b.AcceptUnbound = false
	mustAcceptToken(t, b, tokenNew, "token issued during the transition, after AcceptUnbound was cleared")
	mustRejectToken(t, b, tokenOld, "unbound token after AcceptUnbound was cleared")
}

// TestBindingKeyRotation checks that key rotation still works with Name: a store whose key list
// has moved on accepts a value sealed under an older key with the same Name, and refuses it
// under another Name.
func TestBindingKeyRotation(t *testing.T) {
	_, tokenOld := bindingStore(t, "b", bindingKey1)
	rotated, _ := bindingStore(t, "b", bindingKey2, bindingKey1)
	other, _ := bindingStore(t, "a", bindingKey2, bindingKey1)

	mustAcceptToken(t, rotated, tokenOld, "value sealed under an older key after rotation")
	mustRejectToken(t, other, tokenOld, "token bound to b presented to a after rotation")
}

// TestBindingHooks checks that Name has no effect when Encrypt and Decrypt are supplied: stores
// with different Names and the same hooks accept each other's values.
func TestBindingHooks(t *testing.T) {
	encrypts := 0
	decrypts := 0
	encrypt := func(data []byte) ([]byte, error) {
		encrypts++
		return data, nil
	}
	decrypt := func(data []byte) ([]byte, error) { // Decrypt is given the base64 text of Encrypt's result
		decrypts++
		return base64.URLEncoding.DecodeString(string(data))
	}
	a, _ := bindingStore(t, "a", bindingKey1)
	b, _ := bindingStore(t, "b", bindingKey1)
	a.Encrypt = encrypt
	a.Decrypt = decrypt
	b.Encrypt = encrypt
	b.Decrypt = decrypt

	tokenA := saveToken(t, a)
	saveToken(t, b)
	mustAcceptToken(t, b, tokenA, "token from a store with another Name and the same Encrypt and Decrypt")
	if encrypts == 0 || decrypts == 0 {
		t.Fatal("Encrypt or Decrypt was not called")
	}
}
