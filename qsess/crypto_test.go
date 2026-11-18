// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of Store.Encrypt and Store.Decrypt, which do NOT see package internals

package qsess_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

// TestCustomCryptoRoundTrip checks that Decrypt is given what Encrypt returned, so that a pair of
// functions that invert each other works, for tokens and for cookies.
func TestCustomCryptoRoundTrip(t *testing.T) {
	flip := func(data []byte) ([]byte, error) {
		flipped := make([]byte, len(data))
		for i, b := range data {
			flipped[i] = b ^ 0x5a
		}
		return flipped, nil
	}
	st := tokenStore(t, bindingKey1)
	st.Encrypt = flip
	st.Decrypt = flip

	mustAcceptToken(t, st, saveToken(t, st), "token from a store whose Decrypt inverts its Encrypt")

	st.AuthType = qsess.CookieAuth
	if _, _, err := st.GetSession(httptest.NewRecorder(), saveCookie(t, st)); err != nil {
		t.Fatal("cookie from a store whose Decrypt inverts its Encrypt: rejected - " + err.Error())
	}
}
