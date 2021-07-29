// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qsess implements web sessions, with a user-definable session
// data type, support for cookies and tokens, session revocation by user id,
// and back-ends for goleveldb, Cassandra/Syclla, PostgreSQL, MySQL, and a simple,
// in-memory store.
//
// It is independent of, but integrates easily with, routers,
// middleware frameworks, http.Request.Context(), etc.
// It has zero dependencies beyond the standard library.
// (Database back-ends, which reside in sub-packages, depend on their
// respective database drivers, and any dependencies those drivers require.)
//
//	var qsStore *qsess.Store
//
//	func main() {
//		...
//		qsStore, err = qsess.NewMapStore([]byte("encryption-key------------------"))
//		...
//	}
//
//	func loginHandler(w http.ResponseWriter, r *http.Request) {
//		...
//		// authenticate
//		s := qsStore.NewSession(userID)
//		// fill in session data
//		err := s.Save(w)
//		...
//	}
//
//	func dosomethingHandler(w http.ResponseWriter, r *http.Request) {
//		sess, ttl, err := qsStore.GetSession(w, r)
//		...
//		// if session data has been modified, or it's time to refresh
//			sess.Save(w)
//		...
//	}
//
//	func logoutHandler(w http.ResponseWriter, r *http.Request) {
//		sess, _, err := qsStore.GetSession(w, r)
//		...
//		err = sess.Delete(w)
//		...
//	}
//
// Session data is persisted in the server and accessed via Session.Data.
// (Storing session data only in clients, with a "stateless" back-end,
// is not supported.)
//
// If the only session data you require is a user id, you can ignore
// Session.Data entirely. Just provide the user id as a byte slice to Store.NewSession
// and get it via Session.UserID. Set Store.NewSessData to nil,
// to avoid allocating an empty VarMap for every session.
//
// If you require session data beyond just a user id,
// it is recommended that you supply a data type and serializer,
// using Store.NewSessData. (See qstest/benchmark_test.go
// for examples.) If you do not, the default session data type, VarMap, is a
// map[interface{}]interface{} with gob serialization (which is very slow,
// compared to the alternatives in qstest/benchmark_test.go).
//
// If AuthType is TokenAuth, session references are transmitted to/from
// the client as tokens, rather than cookies. Tokens are opaque,
// base64-encoded strings, but they can be wrapped in structured formats,
// like JWT, by user code.
// To send tokens to clients, user code must either set Store.SendToken
// and Store.DeleteToken or call Session.Token to obtain tokens and manage
// token communication explicitly. To receive tokens from clients, GetSession,
// by default, reads tokens from request headers of the form,
// "Authorization: Bearer <token>". This can be overridden by supplying a
// GetToken callback.
//
// By default, cookies and tokens are encrypted and authenticated, using
// AES-GCM. This can be overridden by supplying Encrypt and Decrypt functions.
//
// Multiple Stores can be used simultaneously. For example, one Store can be
// used to implement login sessions via cookies, while another is used to
// generate and track sign-up email verification tokens.
//
// Sessions are automatically deleted if not Saved within their expiration
// times. This package does not refresh sessions (i.e. reset their
// expiration times), except for the implicit refresh that happens whenever
// Save is called. User code learns the remaining time-to-live whenever it
// calls GetSession, and it can perform a refresh simply by calling Save.
// (See MwRequireSess in package qctx for an example of this.)
//
// When a user changes a password, you can revoke all active sessions for
// the user by calling DeleteByUserID. To use this optional capability,
// you must supply a user id each time you create a session. User ids are
// application-defined and are not interpreted or modified by session code.
// They are persisted to the database and available for the life of
// the session, by calling UserID.
package qsess
