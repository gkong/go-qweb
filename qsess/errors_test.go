// Copyright 2026 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// tests of the classes of the errors that qsess returns, which do NOT see package internals

package qsess_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qstest"
)

// failLaterBackEnd is a back end that keeps nothing and works until err is set, after which each of its
// methods fails with err.
type failLaterBackEnd struct{ err error }

func (b *failLaterBackEnd) Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error {
	if b.err != nil {
		return b.err
	}
	*sessID = []byte{1}
	return nil
}

func (b *failLaterBackEnd) Get(sessID []byte, uID []byte) ([]byte, []byte, int, int, int, error) {
	return []byte{}, []byte{}, 60, 60, 0, b.err
}

func (b *failLaterBackEnd) Consume(sessID []byte, uID []byte) ([]byte, []byte, int, int, int, error) {
	return []byte{}, []byte{}, 60, 60, 0, b.err
}

func (b *failLaterBackEnd) Delete(sessID []byte, uID []byte) error { return b.err }

func (b *failLaterBackEnd) DeleteByUserID(userID []byte) error { return b.err }

// failLaterSession makes a token store on a failLaterBackEnd and returns the back end and the store with a session
// saved in it.
func failLaterSession(t *testing.T) (*failLaterBackEnd, *qsess.Store, *qsess.Session) {
	backEnd := &failLaterBackEnd{}
	st, err := qsess.NewStore(backEnd, false, []byte("key-for-encryption--------------"))
	if err != nil {
		t.Fatal("NewStore failed - " + err.Error())
	}
	st.AuthType = qsess.TokenAuth
	st.NewSessData = nil // this back end keeps no session data
	sess := st.NewSession([]byte("user"))
	if err := sess.Save(nil); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}
	return backEnd, st, sess
}

// driverError stands in for a database driver's error type, which a caller reaches with errors.As.
type driverError struct{ code int }

func (e driverError) Error() string { return fmt.Sprintf("driver error %d", e.code) }

// TestErrorsFromBackEnd checks that GetTokenSession and ConsumeTokenSession report a back end's failure as ErrBackEnd,
// even one that also matches ErrNotFound, and its ErrNotFound as ErrInvalidCredential, with the back end's own error
// still reachable.
func TestErrorsFromBackEnd(t *testing.T) {
	backEnd, st, sess := failLaterSession(t)
	token, _, err := sess.Token()
	if err != nil {
		t.Fatal("Token failed - " + err.Error())
	}

	for _, c := range []struct {
		what  string
		err   error
		class error
	}{
		{"failed", driverError{57}, qsess.ErrBackEnd},
		{"missed", fmt.Errorf("%w - no such row", qsess.ErrNotFound), qsess.ErrInvalidCredential},
		{"missed and failed", errors.Join(fmt.Errorf("cache - %w", qsess.ErrNotFound),
			fmt.Errorf("database - %w", qsess.ErrBackEnd)), qsess.ErrBackEnd},
	} {
		backEnd.err = c.err
		_, _, errGet := st.GetTokenSession(token)
		_, errConsume := st.ConsumeTokenSession(token)
		for call, err := range map[string]error{"GetTokenSession": errGet, "ConsumeTokenSession": errConsume} {
			qstest.MustMatchClass(t, err, c.class, call+" on a back end that "+c.what)
			if !errors.Is(err, c.err) {
				t.Errorf("%s on a back end that %s: the back end's error cannot be reached through errors.Is - %v",
					call, c.what, err)
			}
		}
	}
}

// undecodable is session data that marshals but cannot be unmarshalled, as stored data that is corrupt.
type undecodable struct{ err error }

func (undecodable) Marshal() ([]byte, error) { return []byte{1}, nil }

func (u undecodable) Unmarshal([]byte) error { return u.err }

// TestErrorsUndecodableData checks that stored session data that cannot be decoded is ErrInvalidCredential, so that
// its user logs in again, unless Unmarshal's error matches another class, which it keeps, and that
// ConsumeTokenSession consumes such a session all the same.
func TestErrorsUndecodableData(t *testing.T) {
	st := tokenStore(t, bindingKey1)
	for _, c := range []struct {
		err   error
		class error
	}{
		{errors.New("cannot decode"), qsess.ErrInvalidCredential},
		{fmt.Errorf("key service down - %w", qsess.ErrBackEnd), qsess.ErrBackEnd},
	} {
		st.NewSessData = func() qsess.SessData { return undecodable{c.err} }
		token := saveToken(t, st)

		_, _, err := st.GetTokenSession(token)
		qstest.MustMatchClass(t, err, c.class, "GetTokenSession of undecodable data, "+c.err.Error())
		if !errors.Is(err, c.err) {
			t.Fatalf("GetTokenSession of undecodable data: Unmarshal's error cannot be reached through errors.Is - %v",
				err)
		}
		_, err = st.ConsumeTokenSession(token)
		qstest.MustMatchClass(t, err, c.class, "ConsumeTokenSession of undecodable data, "+c.err.Error())
		if !errors.Is(err, c.err) {
			t.Fatalf("ConsumeTokenSession of undecodable data: Unmarshal's error cannot be reached through "+
				"errors.Is - %v", err)
		}
		if _, err := st.ConsumeTokenSession(token); !errors.Is(err, qsess.ErrNotFound) {
			t.Fatalf("ConsumeTokenSession of undecodable data left the session - %v", err)
		}
	}
}

// TestErrorsGetToken checks the classes of what an application's GetToken reports: no token is
// ErrNoCredential, an error is ErrInvalidCredential unless it already matches a class, which it keeps.
func TestErrorsGetToken(t *testing.T) {
	st := tokenStore(t, bindingKey1)
	r, _ := http.NewRequest("GET", "http://example.com", nil)

	for _, c := range []struct {
		err   error
		class error
		what  string
	}{
		{nil, qsess.ErrNoCredential, "GetToken found no token"},
		{errors.New("malformed header"), qsess.ErrInvalidCredential, "GetToken failed"},
		{fmt.Errorf("token service down - %w", qsess.ErrBackEnd), qsess.ErrBackEnd, "GetToken failed, ErrBackEnd"},
		{qsess.ErrNoCredential, qsess.ErrNoCredential, "GetToken failed, ErrNoCredential"},
		{fmt.Errorf("bad setup - %w", qsess.ErrApplication), qsess.ErrApplication, "GetToken failed, ErrApplication"},
	} {
		st.GetToken = func(w http.ResponseWriter, r *http.Request) (string, error) {
			return "", c.err
		}
		_, _, err := st.GetSession(httptest.NewRecorder(), r)
		qstest.MustMatchClass(t, err, c.class, c.what)
	}
}

// TestErrorsDecrypt checks the classes of what an application's Decrypt reports: an error is
// ErrInvalidCredential unless it already matches a class, which it keeps.
func TestErrorsDecrypt(t *testing.T) {
	st := tokenStore(t, bindingKey1)
	st.Encrypt = func(data []byte) ([]byte, error) { return data, nil }
	token := saveToken(t, st)

	for _, c := range []struct {
		err   error
		class error
		what  string
	}{
		{errors.New("message authentication failed"), qsess.ErrInvalidCredential, "Decrypt failed"},
		{fmt.Errorf("key service down - %w", qsess.ErrBackEnd), qsess.ErrBackEnd, "Decrypt failed, ErrBackEnd"},
	} {
		st.Decrypt = func(data []byte) ([]byte, error) {
			return nil, c.err
		}
		_, _, err := st.GetTokenSession(token)
		qstest.MustMatchClass(t, err, c.class, c.what)
	}
}

// unencodable is session data that cannot be marshalled.
type unencodable struct{}

func (unencodable) Marshal() ([]byte, error) { return nil, errors.New("cannot encode") }

func (unencodable) Unmarshal([]byte) error { return nil }

// TestErrorsApplication checks that a misuse of a store or session, and a failure of one of the application's own
// functions that matches no class, are ErrApplication, and that such a failure keeps a class it does match.
func TestErrorsApplication(t *testing.T) {
	failure := errors.New("application function failed")
	st := tokenStore(t, bindingKey1)
	saved := st.NewSession([]byte("user"))
	if err := saved.Save(nil); err != nil {
		t.Fatal("Save failed - " + err.Error())
	}

	sess := st.NewSession([]byte("user"))
	sess.MaxAgeSecs = 0
	qstest.MustMatchClass(t, sess.Save(nil), qsess.ErrApplication, "Save with a MaxAgeSecs of 0")
	_, _, err := st.NewSession([]byte("user")).Token()
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "Token of a session never saved")

	st.AuthType = qsess.AuthTypeEnum(7)
	qstest.MustMatchClass(t, saved.Save(nil), qsess.ErrApplication, "Save with an AuthType that is neither")
	unsaved := st.NewSession([]byte("user"))
	qstest.MustMatchClass(t, unsaved.Save(nil), qsess.ErrApplication, "a first Save with an AuthType that is neither")
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _, err = st.GetSession(httptest.NewRecorder(), r)
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "GetSession with an AuthType that is neither")
	st.AuthType = qsess.TokenAuth
	_, _, err = unsaved.Token()
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "Token of a session refused for its store's AuthType")

	for _, c := range []struct {
		what  string
		err   error
		class error
		set   func(err error)
	}{
		{"SendToken failed", failure, qsess.ErrApplication, func(err error) {
			st.SendToken = func(string, int, http.ResponseWriter) error { return err }
		}},
		{"SendToken failed, ErrBackEnd", fmt.Errorf("token service down - %w", qsess.ErrBackEnd), qsess.ErrBackEnd,
			func(err error) { st.SendToken = func(string, int, http.ResponseWriter) error { return err } }},
		{"SendToken failed, ErrInvalidCredential", fmt.Errorf("client gone - %w", qsess.ErrInvalidCredential),
			qsess.ErrInvalidCredential,
			func(err error) { st.SendToken = func(string, int, http.ResponseWriter) error { return err } }},
		// only the back end's ErrNotFound means that the session is gone
		{"SendToken failed, ErrNotFound", fmt.Errorf("no such client - %w", qsess.ErrNotFound), qsess.ErrApplication,
			func(err error) { st.SendToken = func(string, int, http.ResponseWriter) error { return err } }},
		{"SessionSaved failed", failure, qsess.ErrApplication, func(err error) {
			st.SessionSaved = func([]byte, time.Time) error { return err }
		}},
		{"Encrypt failed", failure, qsess.ErrApplication, func(err error) {
			st.Encrypt = func([]byte) ([]byte, error) { return nil, err }
			st.SendToken = func(string, int, http.ResponseWriter) error { return nil }
		}},
	} {
		c.set(c.err)
		err := saved.Save(nil)
		qstest.MustMatchClass(t, err, c.class, "Save, "+c.what)
		if !errors.Is(err, c.err) {
			t.Fatalf("Save, %s: the application's error cannot be reached through errors.Is - %v", c.what, err)
		}
		st.SendToken = nil
		st.SessionSaved = nil
		st.Encrypt = nil
	}

	st.Encrypt = func([]byte) ([]byte, error) { return nil, failure }
	_, _, err = saved.Token()
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "Token, Encrypt failed")
	st.AuthType = qsess.CookieAuth
	err = saved.Save(httptest.NewRecorder())
	qstest.MustMatchClass(t, err, qsess.ErrApplication, "Save to a cookie, Encrypt failed")
	st.AuthType = qsess.TokenAuth
	st.Encrypt = nil

	st.NewSessData = func() qsess.SessData { return unencodable{} }
	qstest.MustMatchClass(t, st.NewSession([]byte("user")).Save(nil), qsess.ErrApplication, "Save, Marshal failed")

	both := failDeleteStore(t, errDBDelete)
	both.AuthType = qsess.TokenAuth
	both.DeleteToken = func(http.ResponseWriter) error { return failure }
	err = savedSession(t, both).Delete(nil)
	if !errors.Is(err, qsess.ErrBackEnd) || !errors.Is(err, qsess.ErrApplication) {
		t.Fatalf("Delete whose database delete and DeleteToken both failed: err = %v, want both classes", err)
	}
}

// TestErrorsSessionMethods checks that Save, Token, Delete and DeleteByUserID report a back end's failure as
// ErrBackEnd, with the back end's own error still reachable; that Save and Token report a session whose record
// is gone as ErrInvalidCredential; and that DeleteByUserID, like Delete, takes a back end's miss as success,
// unless its error also matches a class, which it keeps, as every other call does.
func TestErrorsSessionMethods(t *testing.T) {
	backEnd, _, sess := failLaterSession(t)

	backEnd.err = driverError{57}
	_, _, errToken := sess.Token()
	for _, c := range []struct {
		what string
		err  error
	}{
		{"Save", sess.Save(nil)},
		{"Token", errToken},
		{"Delete", sess.Delete(nil)},
		{"DeleteByUserID", sess.DeleteByUserID(nil)},
	} {
		qstest.MustMatchClass(t, c.err, qsess.ErrBackEnd, c.what+" on a failed back end")
		var de driverError
		if !errors.As(c.err, &de) || de.code != 57 {
			t.Fatalf("%s on a failed back end: the back end's error cannot be reached through errors.As - %v",
				c.what, c.err)
		}
	}

	backEnd.err = fmt.Errorf("%w - no such row", qsess.ErrNotFound)
	err := sess.Save(nil)
	qstest.MustMatchClass(t, err, qsess.ErrInvalidCredential, "Save of a session whose record is gone")
	if !errors.Is(err, qsess.ErrNotFound) {
		t.Fatal("Save of a session whose record is gone: not reported as a back-end miss - " + err.Error())
	}
	_, _, err = sess.Token()
	qstest.MustMatchClass(t, err, qsess.ErrInvalidCredential, "Token of a session whose record is gone")
	if err := sess.DeleteByUserID(nil); err != nil {
		t.Fatal("DeleteByUserID failed for a user id whose sessions are gone - " + err.Error())
	}

	backEnd.err = errors.Join(fmt.Errorf("cache - %w", qsess.ErrNotFound),
		fmt.Errorf("database - %w", qsess.ErrBackEnd))
	_, _, errToken = sess.Token()
	for _, c := range []struct {
		what string
		err  error
	}{
		{"Save", sess.Save(nil)},
		{"Token", errToken},
		{"Delete", sess.Delete(nil)},
		{"DeleteByUserID", sess.DeleteByUserID(nil)},
	} {
		qstest.MustMatchClass(t, c.err, qsess.ErrBackEnd, c.what+" on a back end that missed and failed")
	}
}
