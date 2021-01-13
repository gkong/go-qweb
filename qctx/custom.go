// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Edit this file, to customize Ctx and remove any unwanted dependencies.

package qctx

import (
	"net/http"

	"github.com/gkong/go-qweb/qsess"
	"github.com/julienschmidt/httprouter"
)

// Ctx holds per-request ("context") information.
// To add your own data, fork this code and add members to the Ctx struct.
type Ctx struct {
	W http.ResponseWriter
	R *http.Request

	// the following are optional and can be eliminated if customizing Ctx
	Params httprouter.Params // only used if using julienschmidt/httprouter
	Sess   *qsess.Session    // only used if using github.com/gkong/go-qweb/qsess
}

// Hr2Ctx adapts a CtxHandler to be callable by julienschmidt/httprouter.
func Hr2Ctx(next CtxHandler) httprouter.Handle {
	return httprouter.Handle(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		c := &Ctx{R: r, W: w, Params: p}
		next.CtxServeHTTP(c)
	})
}

// HRHandle combines a middleware stack and a final handler function into an
// httprouter.Handle, ready to be registered with julienschmidt/httprouter.
func (s MakerStack) HRHandle(f func(*Ctx)) httprouter.Handle {
	var h CtxHandler = CtxHandlerFunc(f)
	for i := len(s) - 1; i >= 0; i-- {
		h = s[i](h)
	}
	return Hr2Ctx(h)
}

// MwRequireSess is middleware which checks for a valid qsess session.
// If it finds one, it calls downstream, otherwise, it returns an error.
// It also refreshes session expiration time, if needed.
func MwRequireSess(st *qsess.Store) MwMaker {
	return func(next CtxHandler) CtxHandler {
		return CtxHandlerFunc(func(c *Ctx) {
			var ttl int
			var err error
			c.Sess, ttl, err = st.GetSession(c.W, c.R)
			if err != nil {
				c.Error("not logged in", http.StatusUnauthorized)
				return
			}
			if (c.Sess.MinRefreshSecs >= 0) && (ttl < (c.Sess.MaxAgeSecs - c.Sess.MinRefreshSecs)) {
				// do this before calling downstream, because we return
				// cookies to the client in http headers, which we can't
				// do later, if ResponseWriter.WriteHeader has been called.
				c.Sess.Save(c.W)
			}
			next.CtxServeHTTP(c)
		})
	}
}
