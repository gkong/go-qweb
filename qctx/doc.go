// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Package qctx is a light-weight, type-safe, per-http-request
// state manager, with simple middleware stacking and a few middleware
// and helper functions.
//
// Request-scoped data is kept in a Ctx struct, which is passed as an
// argument to handlers and middleware.
// Package qctx defines its own signature for Middleware functions,
// but middleware written to the standard library http.Handler interface
// can be included via a wrapper. See MwStripPrefix for an example of this.
//
// Support is included for net/http, julienschmidt/httprouter and
// github.com/gkong/go-qweb/qsess.
//
// User data can flow through middleware and handlers in several ways:
// (1) struct Ctx contains a qsess.Session, which (if you choose to use qsess)
// can store user data for the duration of a session, (2) you can fork
// this package and add your own members to struct Ctx - an easy and type-safe
// way to share data for the duration of a request (if you fork, the code is
// organized to make it very easy to remove the dependencies on httprouter
// and qsess, thus making qctx a zero-dependency package), and (3) struct Ctx
// contains an http.Request, which (in go 1.7 or later) contains a
// Context, which can store request-scoped user data, enabling intermixture
// of third-party middleware that uses http.Request.WithContext and
// http.Request.Context (through the use of a simple wrapper).
//
// Handler:
//		func myHandler(c *qctx.Ctx) {
//			fmt.Fprint(c.W, "Hello, world!")
//		}
//
// Middleware:
//		func myMiddleware(next CtxHandler) CtxHandler {
//			return CtxHandlerFunc(func(c *Ctx) {
//				// do stuff before calling downstream
//				next.CtxServeHTTP(c)
//				// do stuff after returning from downstream
//			})
//		}
//
// Middleware Stacking:
//		mw := qctx.MwStack(qctx.MwRecovery(os.Stderr), qctx.MwLogger(os.Stderr, true))
//		mw = mw.Append(myMiddleware)
//
// Use with net/http:
//		http.Handle("/hello", mw.Handle(myHandler)))
//
// Use with julienschmidt/httprouter:
//		r := httprouter.New()
//		r.GET("/hello", mw.HRHandle(myHandler)))
package qctx
