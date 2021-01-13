// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qctx

import (
	"net/http"
)

// CtxHandler is a handler/middleware interface like http.Handler,
// but with a single ctx argument.
type CtxHandler interface {
	CtxServeHTTP(*Ctx)
}

type CtxHandlerFunc func(*Ctx)

func (f CtxHandlerFunc) CtxServeHTTP(c *Ctx) {
	f(c)
}

// H2Ctx adapts a CtxHandler to be callable by net/http.
func H2Ctx(next CtxHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := &Ctx{R: r, W: w}
		next.CtxServeHTTP(c)
	})
}

// middleware stacking, inspired by justinas/alice

// MwMaker defines a piece of stackable middleware.
// Given a function to be called next, it returns a handler function,
// which performs a middleware task and calls the next function,
// which it remembers by closing over the MwMaker's "next" parameter.
type MwMaker func(next CtxHandler) CtxHandler

type MakerStack []MwMaker

// MwStack makes a middleware stack from one or more MwMakers.
func MwStack(makers ...MwMaker) MakerStack {
	return makers
}

// Append returns a new MakerStack, composed of the contents of an existing
// MakerStack and some additional MwMakers.
// It does not modify the existing MakerStack.
func (s MakerStack) Append(makers ...MwMaker) MakerStack {
	result := make([]MwMaker, len(s), len(s)+len(makers))
	copy(result, s)
	return append(result, makers...)
}

// Handle combines a middleware stack and a final handler function
// into an http.Handler, ready to be registered with net/http.
func (s MakerStack) Handle(f func(*Ctx)) http.Handler {
	// Go thru the stack in reverse order, calling each maker,
	// to make a closure whose "next" function is the previous outermost.
	// Then call H2Ctx to  make a final (outermost) closure, which will be
	// the first to be called and which handles context allocation.
	var h CtxHandler = CtxHandlerFunc(f)
	for i := len(s) - 1; i >= 0; i-- {
		h = s[i](h)
	}
	return H2Ctx(h)
}
