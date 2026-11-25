// Copyright 2026 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsess

import (
	"errors"
	"fmt"
)

// Each error that GetSession, GetTokenSession, ConsumeTokenSession and the Session methods Save, Token, Delete and
// DeleteByUserID return matches exactly one of these, as errors.Is reports it, except that a Delete whose database
// delete and DeleteToken both fail reports both, and so matches both their classes. An error from one of the
// application's own functions keeps the class it matches, if any; one that matches none is ErrInvalidCredential
// from GetToken, Decrypt or SessData's Unmarshal, and ErrApplication from any other.
var (
	// ErrNoCredential means the request carries no cookie or token for the store.
	ErrNoCredential = errors.New("qsess: no cookie or token")

	// ErrInvalidCredential means the cookie or token does not decrypt, or names a session that is missing, revoked
	// or expired, or one whose data cannot be decoded; from a Session's method, that the session is gone or has
	// expired.
	ErrInvalidCredential = errors.New("qsess: invalid cookie or token")

	// ErrBackEnd means the back end failed.
	ErrBackEnd = errors.New("qsess: back end failed")

	// ErrApplication means the application used a store or session in a way the documentation rules out, or one
	// of its own functions failed, so that neither the client nor the back end is at fault.
	ErrApplication = errors.New("qsess: application error")
)

// ErrNotFound is matched by a back end's error for a session id that has no live record (see SessBackEnd).
var ErrNotFound = errors.New("qsess: session not found")

// classify gives err the class def, unless err already matches one of the four classes.
func classify(def error, err error) error {
	if hasClass(err) {
		return err
	}
	return fmt.Errorf("%w - %w", def, err)
}

// hasClass reports whether err matches one of the four classes.
func hasClass(err error) bool {
	return errors.Is(err, ErrNoCredential) || errors.Is(err, ErrInvalidCredential) || errors.Is(err, ErrBackEnd) ||
		errors.Is(err, ErrApplication)
}

// classifyBackEnd classifies an error from the back end, whose ErrNotFound, unlike an application function's,
// means that the session is gone.
func classifyBackEnd(err error) error {
	if errors.Is(err, ErrNotFound) {
		return classify(ErrInvalidCredential, err)
	}
	return classify(ErrBackEnd, err)
}

// isGone reports whether err, from the back end, says that the session is gone and matches no class, which a
// delete takes as success, because classifyBackEnd lets a class an error matches win over ErrNotFound.
func isGone(err error) bool {
	return errors.Is(err, ErrNotFound) && !hasClass(err)
}
