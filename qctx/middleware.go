// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qctx

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

// WrappedRW wraps http.ResponseWriter, to capture the HTTP status code and
// observe whether or not any data has been written to the response body.
type WrappedRW struct {
	http.ResponseWriter
	WroteHeader bool // headers and code written, can no longer be changed
	WroteBody   bool // have written data to the response body
	StatusCode  int  // http status code, only meaningful if wroteHeader true
}

func (w *WrappedRW) WriteHeader(code int) {
	if !w.WroteHeader {
		// only the first one counts
		w.StatusCode = code
		w.WroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *WrappedRW) Write(data []byte) (int, error) {
	if !w.WroteBody {
		if len(data) > 0 {
			w.WroteBody = true
		}
	}
	return w.ResponseWriter.Write(data)
}

// MwRecovery is panic-recovery middleware in the spirit of http.Error() -
// it assumes the client, upon receipt of an error code, expects a plain-text
// error message in the response body. If it is unable to reply in that manner,
// because the status code has already been set, or some data has already been
// written to the response body, it panics again, which results in the client
// seeing a dropped connection (which is better than a false indication
// of success or an incomprehensible error message).
func MwRecovery(log io.Writer, printStack bool, printAll bool) MwMaker {
	return func(next CtxHandler) CtxHandler {
		return CtxHandlerFunc(func(c *Ctx) {
			// wrap W with WrappedRW (if not already wrapped)
			wrapped, ok := c.W.(*WrappedRW)
			if !ok {
				wrapped = &WrappedRW{c.W, false, false, 0}
				c.W = wrapped
			}

			defer func() {
				if x := recover(); x != nil {
					fmt.Fprintf(log, "MwRecovery - PANIC - %s %s - %v\n",
						c.R.RemoteAddr, c.R.URL.String(), x)
					if printStack {
						bsize := 10000
						if printAll {
							bsize = 1 << 20
						}
						b := make([]byte, bsize)
						n := runtime.Stack(b, printAll)
						fmt.Fprintf(log, "%s", b[0:n])
					}
					if !wrapped.WroteHeader && !wrapped.WroteBody {
						c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
						c.Error("!MwRecovery - PANIC", http.StatusInternalServerError)
						return
					}
					panic(http.ErrAbortHandler)
				}
			}()

			next.CtxServeHTTP(c)
		})
	}
}

// MwLogger is middleware which logs HTTP requests to a given io.Writer,
// optionally including decoration with ANSI escape sequences.
//
// It wraps http.ResponseWriter, to capture the response status code, so it
// should be placed upstream of any middleware that could generate a
// status code.
func MwLogger(log io.Writer, useAnsiColor bool) MwMaker {
	ansiDefault := ""
	if useAnsiColor {
		ansiDefault = "\033[0m" // set color to default
	}
	return func(next CtxHandler) CtxHandler {
		return CtxHandlerFunc(func(c *Ctx) {
			start := time.Now()

			// save stuff to print, in case anybody downstream monkeys with it
			method := c.R.Method
			url := c.R.URL.String()
			remote := c.R.RemoteAddr

			// wrap W with WrappedRW (if not already wrapped)
			wrapped, ok := c.W.(*WrappedRW)
			if !ok {
				wrapped = &WrappedRW{c.W, false, false, 0}
				c.W = wrapped
			}

			next.CtxServeHTTP(c)

			code := http.StatusOK
			if wrapped.WroteHeader {
				code = wrapped.StatusCode
			}

			ansiColor := ""
			if useAnsiColor {
				ansiColor = chooseColor(code)
			}
			fmt.Fprintf(log, "%45s %9d us %s %3d %s %7s %s\n", remote,
				time.Now().Sub(start).Nanoseconds()/1000,
				ansiColor, code, ansiDefault, method, url)
		})
	}
}

func chooseColor(code int) string {
	switch {
	case code >= 200 && code <= 299:
		return "\033[42;37m" // white on green
	case code >= 300 && code <= 399:
		return "\033[38;5;238m\033[48;5;228m" // dark grey on light yellow
	case code >= 400 && code <= 499:
		return "\033[38;5;15m\033[48;5;166m" // white on orange
	case code >= 500 && code <= 599:
		return "\033[41;37m" // white on red
	default:
		return ""
	}
}

// MwHeader is middleware which sets HTTP response headers.
// Pass it one or more key/value pairs.
func MwHeader(s ...string) MwMaker {
	if len(s)%2 == 1 {
		panic("MwHeader: odd number of parameters")
	}
	return func(next CtxHandler) CtxHandler {
		return CtxHandlerFunc(func(c *Ctx) {
			for i := 0; i < len(s); i += 2 {
				c.W.Header().Set(s[i], s[i+1])
			}
			next.CtxServeHTTP(c)
		})
	}
}

// MwStripPrefix is middleware that removes a given prefix from the request
// URL's path, by wrapping the standard go library http.StripPrefix.
// It is an example of how to incorporate http.Handler style middleware
// into a stack of CtxHandlers.
func MwStripPrefix(prefix string) MwMaker {
	return func(next CtxHandler) CtxHandler {
		return CtxHandlerFunc(func(c *Ctx) {
			doNext := func(w http.ResponseWriter, r *http.Request) {
				next.CtxServeHTTP(c)
			}
			http.StripPrefix(prefix, http.HandlerFunc(doNext)).ServeHTTP(c.W, c.R)
		})
	}
}
