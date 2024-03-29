// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// implementation notes:
//
// Session expiration is implemented by maintaining a "time-to-live" for each
// session and having database back ends delete expired sessions
// (thus keeping expiration management within a single time domain).
//
// DeleteByUserID requires that qsess maintain a user id and back-ends
// perisist it for the life of the session. Back-ends must also be able to
// locate all the active sessions for a given user id efficiently, which
// typically is accomplished by maintaining an index of sessions by user id.

package qsess

import (
	"crypto/cipher"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AuthTypeEnum specifies how references to sessions are stored in clients -
// in cookies or in tokens.
type AuthTypeEnum int

const (
	CookieAuth AuthTypeEnum = iota
	TokenAuth
)

func (a AuthTypeEnum) String() string {
	switch a {
	case CookieAuth:
		return "cookie"
	case TokenAuth:
		return "token"
	default:
		return "!UNDEFINED-AuthTypeEnum"
	}
}

func ParseAuthType(s string) (AuthTypeEnum, error) {
	var at AuthTypeEnum
	switch strings.ToLower(s) {
	case "cookie":
		at = CookieAuth
	case "token":
		at = TokenAuth
	default:
		return 0, fmt.Errorf("qsess.parseAuthType - < %s > is not a valid auth type", s)
	}
	return at, nil
}

// A back-end consists of an implementation of SessBackEnd and a Store constructor.
// Session ids are back-end-defined and generated by SessBackEnd.Save,
// are not interpreted or modified by code outside of the back-end,
// and are not visible to users.
// User ids are application-defined. Back-ends track them for DeleteByUserId.
type SessBackEnd interface {
	// Save might have to create and save a new id, so id is passed by reference.
	Save(sessID *[]byte, data []byte, userID []byte, maxAgeSecs int, minRefreshSecs int) error
	// Get and Delete take a userID argument, but it is ignored except in
	// the rare case of a back-end that requires uidToClient = true.
	Get(sessID []byte, uID []byte) (data []byte, userID []byte, timeToLiveSecs int, maxAgeSecs int, minRefreshSecs int, err error)
	Delete(sessID []byte, uID []byte) error
	DeleteByUserID(userID []byte) error
}

// SessData is an interface for per-session data storage.
// The default session data type is VarMap.
// It can be replaced with a custom data type by setting Store.NewSessData.
// See qstest/benchmark_test.go for examples.
type SessData interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

type Store struct {
	// To use a custom session data type, register a constructor for an
	// implementation of SessData here.
	// NewSessData can also be set to nil, if the only session data you need
	// is a user id (which can be managed via NewSession and UserID).
	NewSessData func() SessData

	// SessMaxAgeSecs is the session expiration period. It must be positive.
	// It can be overridden in individual sessions by setting Session.MaxAgeSecs.
	MaxAgeSecs int

	// SessMinRefreshSecs enables applications to reduce refresh overhead,
	// by not automatically refreshing the session expiration time at
	// every request. Package qsess does not perform session refresh itself;
	// it maintains this value for use by applications and/or middleware.
	// For a usage example, see qctx.MwRequireSess.
	// It can be overridden in individual sessions via Session.MinRefreshSecs.
	// By convention, if MinRefreshSecs is negative, no refresh should be performed.
	MinRefreshSecs int

	// parameters for cookie creation
	CookieName     string
	CookieDomain   string
	CookiePath     string
	CookieSecure   bool
	CookieHTTPOnly bool
	CookieSameSite http.SameSite

	// AuthType specifies how sessions are stored in the client (cookies or tokens).
	AuthType AuthTypeEnum

	// callbacks for sending/receiving tokens to/from the client.
	SendToken   func(token string, timeToLiveSecs int, w http.ResponseWriter) error
	DeleteToken func(w http.ResponseWriter) error
	GetToken    func(w http.ResponseWriter, r *http.Request) (token string, err error)

	// Bring-your-own-crypto by registering Encrypt and Decrypt functions.
	Encrypt func(data []byte) ([]byte, error)
	Decrypt func(data []byte) ([]byte, error)

	// SessionSaved is an optional callback, which enables you to keep track
	// of users' last-visited time. This has no effect on session expiration;
	// it is purely for use by application code. You must supply a userID
	// with NewSession, for this to be useful.
	SessionSaved func(UserID []byte, timestamp time.Time) error

	// for back-ends that create a goroutine to prune expired sessions
	PruneInterval chan int // value is interval in seconds
	PruneKill     chan int // kill pruner goroutine, value doesn't matter

	backEnd SessBackEnd

	// true if back-end requires user id (in addition to session id) to locate
	// sessions, so cookies/tokens must store both user id and session id.
	// NOTE: this exposes (encrypted) user ids to clients.
	uidToClient bool

	// when encrypting: always use ciphers[0]. when decrypting - try all ciphers
	ciphers []cipher.AEAD
}

const (
	DefaultAuthType       = CookieAuth
	DefaultMaxAgeSecs     = 24 * 60 * 60
	DefaultMinRefreshSecs = 1 * 60 * 60
	DefaultCookieName     = "qsess_session"
	DefaultCookieDomain   = ""
	DefaultCookiePath     = "/"
	DefaultCookieSecure   = false
	DefaultCookieHTTPOnly = true
	DefaultCookieSameSite = http.SameSiteDefaultMode
)

// NewStore is exported only for use by back-ends.
// Users should never call NewStore; instead, they should call
// back-end-specific Store constructors.
func NewStore(backend SessBackEnd, uidToClient bool, cipherkeys ...[]byte) (*Store, error) {
	if len(cipherkeys) == 0 {
		return nil, qsErr{"NewStore - must have at least one cipherkey", nil}
	}

	st := &Store{
		AuthType:       DefaultAuthType,
		MaxAgeSecs:     DefaultMaxAgeSecs,
		MinRefreshSecs: DefaultMinRefreshSecs,
		CookieName:     DefaultCookieName,
		CookieDomain:   DefaultCookieDomain,
		CookiePath:     DefaultCookiePath,
		CookieSecure:   DefaultCookieSecure,
		CookieHTTPOnly: DefaultCookieHTTPOnly,
		CookieSameSite: DefaultCookieSameSite,
		NewSessData:    newVarMap,

		backEnd:     backend,
		uidToClient: uidToClient,
		ciphers:     make([]cipher.AEAD, len(cipherkeys)),
	}

	return st, st.makeCiphers(cipherkeys...)
}

type Session struct {
	Data SessData
	// MaxAgeSecs and MinRefreshSecs are initialized to the corresponding
	// values in the Store. User code may set them to different values,
	// for example, to implement a "keep me logged in" option.
	// See Store for more information on how these values are used.
	MaxAgeSecs     int
	MinRefreshSecs int
	// sessId is a back-end-defined database key.
	// In newly-created sessions, it is empty, until BackEnd.Save() fills it in.
	sessID []byte
	// userId is application-defined and maintained for DeleteByUserId.
	userID []byte
	store  *Store
}

// NewSession creates a new session object.
// It is not persisted to until Session.Save() is called.
//
// userID is an application-defined user id.
// If you are using DeleteByUserId, you must supply a nonempty userID.
// Even if you are not using DeleteByUserId, you may supply a userID,
// and it will be persisted by the back-end for the life of the session
// and will be accessible by calling UserID.
// Otherwise, userID can be empty or nil.
func (st *Store) NewSession(userID []byte) *Session {
	if userID == nil {
		userID = []byte{}
	}
	s := st.newSess()
	s.userID = userID
	return s
}

func (st *Store) newSess() *Session {
	var d SessData
	if st.NewSessData != nil {
		d = st.NewSessData()
	} else {
		d = noSessData{}
	}

	return &Session{
		Data:           d,
		MaxAgeSecs:     st.MaxAgeSecs,
		MinRefreshSecs: st.MinRefreshSecs,
		store:          st,
	}
}

// GetSession determines if the current HTTP request headers contain a cookie
// or token for an active session and, if so, returns a valid *Session,
// otherwise it returns a non-nil error.
func (st *Store) GetSession(w http.ResponseWriter, r *http.Request) (s *Session, timeToLiveSecs int, e error) {
	var idEncrypted string

	switch st.AuthType {
	case CookieAuth:
		cookie, err := r.Cookie(st.CookieName)
		if err != nil {
			return nil, 0, qsErr{"GetSession - no cookie", err}
		}
		idEncrypted = cookie.Value
	case TokenAuth:
		if st.GetToken != nil {
			tok, err := st.GetToken(w, r)
			if err != nil {
				return nil, 0, qsErr{"GetSession - GetToken failed", err}
			}
			idEncrypted = tok
		} else {
			tok := r.Header.Get("Authorization")
			if len(tok) < 8 || strings.ToLower(tok[0:7]) != "bearer " {
				return nil, 0, qsErr{"GetSession - token not present or malformed", nil}
			}
			idEncrypted = tok[7:]
		}
	}

	return st.GetTokenSession(idEncrypted)
}

// GetTokenSession determines if the given token refers to an active session
// and, if so, returns a valid *Session, otherwise it returns a non-nil error.
func (st *Store) GetTokenSession(token string) (s *Session, timeToLiveSecs int, e error) {
	s = st.newSess()

	if err := s.decode(token); err != nil {
		return nil, 0, qsErr{"GetTokenSession - decode - ", err}
	}

	dbData, userid, ttl, maxage, minrefresh, err := st.backEnd.Get(s.sessID, s.userID)
	if err != nil {
		return nil, 0, qsErr{"GetTokenSession - no record in db", err}
	}
	if err := s.Data.Unmarshal(dbData); err != nil {
		return nil, 0, qsErr{"GetTokenSession - unmarshal failed", err}
	}

	s.MaxAgeSecs = maxage
	s.MinRefreshSecs = minrefresh
	s.userID = userid
	return s, ttl, nil
}

// Token returns a token referring to the current session, ready to be given to the client.
func (s *Session) Token() (token string, timeToLiveSecs int, err error) {
	_, _, ttl, _, _, err := s.store.backEnd.Get(s.sessID, s.userID)
	if err != nil {
		return "", 0, qsErr{"Token - Get failed", err}
	}
	token, err = s.encode()
	if err != nil {
		return "", 0, qsErr{"Token - token creation failed", err}
	}
	return token, ttl, nil
}

// UserID returns the user id that was specified when the session was created.
// Callers should NOT modify the contents of the returned byte slice.
func (s *Session) UserID() []byte {
	// XXX - consider trading GC pressure for safety, by making a copy...
	return s.userID
}

// Save writes a session's data to the database and refreshes its
// expiration time.
//
// If AuthType is CookieAuth, Save writes a cookie containing the session id
// into the response headers. A call to Save must precede any calls to
// ResponseWriter.Write or ResponseWriter.WriteHeader, otherwise, the cookie
// will not make it to the client.
//
// If AuthType is TokenAuth, you must either supply an implementation of
// SendToken or make other arrangements for sending the token to the client.
func (s *Session) Save(w http.ResponseWriter) error {
	st := s.store

	if s.MaxAgeSecs < 1 {
		return qsErr{"Save - MaxAgeSecs must be positive", nil}
	}

	dbData, err := s.Data.Marshal()
	if err != nil {
		return qsErr{"Save - marshal failed", err}
	}
	err = st.backEnd.Save(&s.sessID, dbData, s.userID, s.MaxAgeSecs, s.MinRefreshSecs)
	if err != nil {
		return qsErr{"Save - db write failed", err}
	}

	switch st.AuthType {
	case CookieAuth:
		ckData, err := s.encode()
		if err != nil {
			return qsErr{"Save - cookie encode failed", err}
		}
		http.SetCookie(w, s.newCookie(ckData))
	case TokenAuth:
		if st.SendToken != nil {
			tokData, err := s.encode()
			if err != nil {
				return qsErr{"Save - token creation failed", err}
			}
			if err := st.SendToken(tokData, s.MaxAgeSecs, w); err != nil {
				return qsErr{"Save - SendToken failed", err}
			}
		}
	}

	if st.SessionSaved != nil {
		if err := st.SessionSaved(s.userID, time.Now()); err != nil {
			return qsErr{"Save - SessionSaved failed", err}
		}
	}

	return nil
}

// Delete deletes a session, by deleting its database record.
// If you're using cookies, the corresponding cookie is also deleted.
// If you're using tokens, and you've registered an implementation of
// DeleteToken, Delete will call it to delete the token from the cient.
func (s *Session) Delete(w http.ResponseWriter) error {
	// attempt to delete from database and from client.
	// if either one succeeds, the session is effectively deleted.
	// if a zombie database entry is left, back-end will eventually prune it.
	errDb := s.store.backEnd.Delete(s.sessID, s.userID)
	errClient := s.deleteFromClient(w)

	// if BOTH failed, return an error.
	if errDb != nil && errClient != nil {
		return qsErr{"Delete - database - " + errDb.Error() + " --AND-- client - ", errClient}
	}

	return nil
}

func (s *Session) deleteFromClient(w http.ResponseWriter) error {
	st := s.store

	switch st.AuthType {
	case CookieAuth:
		s.deleteCookie(w)
	case TokenAuth:
		if st.DeleteToken != nil {
			if err := st.DeleteToken(w); err != nil {
				return qsErr{"deleteFromClient - DeleteToken failed", err}
			}
		}
	}

	return nil
}

// DeleteByUserID deletes all sessions for the given session's user id.
// If you are going to use DeleteByUserID, you must supply non-empty
// userIDs to NewSession.
//
// It's OK to invoke DeleteByUserID on a newly-created session object,
// on which Save has never been called.
func (s *Session) DeleteByUserID(w http.ResponseWriter) error {
	st := s.store

	// delete all with matching userID (including the current session)
	errDb := st.backEnd.DeleteByUserID(s.userID)

	// delete current session from client (but not if has never been Saved)
	if s.sessID != nil {
		s.deleteFromClient(w)
	}

	if errDb != nil {
		return qsErr{"DeleteByUserID - back-end - ", errDb}
	}
	return nil
}

// given a Session, return data ready to send to client in a cookie or token.
// if Store.uidToClient is false, just encrypt the session id.
// if it is true, marshall session id + user id, then encrypt.
func (s *Session) encode() (string, error) {
	var data []byte
	var err error
	if s.store.uidToClient {
		// marshall session id and user id into a buffer, then encrypt
		sidlen := len(s.sessID)
		uidlen := len(s.userID)
		if sidlen > 255 {
			return "", qsErr{"encode - session id too big", nil}
		}
		buf := make([]byte, sidlen+uidlen+1)
		buf[0] = byte(sidlen)
		copy(buf[1:1+sidlen], s.sessID)
		copy(buf[1+sidlen:], s.userID)
		data, err = s.store.encrypt(buf)
	} else {
		// just encrypt the session id
		data, err = s.store.encrypt(s.sessID)
	}
	if err != nil {
		return "", qsErr{"encode - encrypt failed", err}
	}
	return string(data), nil
}

// decode cookie/token data into a Session's session id (and possibly user id)
func (s *Session) decode(token string) error {
	data, err := s.store.decrypt([]byte(token))
	if err != nil {
		return qsErr{"decode - decrypt - ", err}
	}
	if s.store.uidToClient {
		// unmarshall session id and user id from decrypted data
		sidlen := int(data[0])
		if sidlen+1 > len(data) {
			return qsErr{"decode - bad data from client", nil}
		}
		s.sessID = data[1 : 1+sidlen]
		s.userID = data[1+sidlen:]
	} else {
		// decrypted data is just the session id
		s.sessID = data
	}
	return nil
}

func (s *Session) newCookie(value string) *http.Cookie {
	// Use MaxAge, not Expires, to avoid client/server time sync issues.

	st := s.store
	c := &http.Cookie{
		Name:     st.CookieName,
		Value:    value,
		Path:     st.CookiePath,
		Domain:   st.CookieDomain,
		MaxAge:   s.MaxAgeSecs,
		Secure:   st.CookieSecure,
		HttpOnly: st.CookieHTTPOnly,
		SameSite: st.CookieSameSite,
	}

	return c
}

func (s *Session) deleteCookie(w http.ResponseWriter) {
	// to delete a cookie from the client, send one with a negative MaxAge
	cookie := s.newCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
}

// BackEnd is exported only for use by tests.
func (st *Store) BackEnd() SessBackEnd {
	return st.backEnd
}

// error message wrapping with context and lazy evaluation

type qsErr struct {
	msg string
	err error
}

func (e qsErr) Error() string {
	if e.err != nil {
		return "qsess." + e.msg + " - " + e.err.Error()
	}
	return "qsess." + e.msg
}
