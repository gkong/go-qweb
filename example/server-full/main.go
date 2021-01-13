// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Restful back end to exercise and load test qsess and qctx and experiment
// with web app ideas.
package main

import (
	"html"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"time"
	"unicode/utf8"

	. "github.com/gkong/go-qweb/example/api"
	"github.com/gkong/go-qweb/qctx"
	"github.com/gkong/go-qweb/qsess"

	"github.com/julienschmidt/httprouter"
)

func main() {
	doConfig()
	defer dbClose()

	// middleware stacks
	root := qctx.MwStack(qctx.MwRecovery(os.Stderr, true, false))
	if Config.Logging {
		root = root.Append(qctx.MwLogger(os.Stderr, true))
	}
	// content type is text/plain by default and is overridden by qctx.WriteJSON
	plain := root.Append(qctx.MwHeader("Content-Type", "text/plain; charset=utf-8"))
	sess := plain.Append(qctx.MwRequireSess(qsStore))

	r := httprouter.New()

	// We're a single-page app, so all URLs that are not static files or
	// API endpoints return index.html and let front end decide what to do.
	r.NotFound = root.Handle(func(c *qctx.Ctx) { http.ServeFile(c.W, c.R, "../../example/client/index.html") })

	r.ServeFiles("/static/*filepath", http.Dir("../../example/client/static"))

	r.GET("/authtype", plain.HRHandle(authTypeHandler))
	r.POST("/login", root.HRHandle(loginHandler))
	r.GET("/fetch", sess.HRHandle(fetchHandler))
	r.POST("/update", sess.HRHandle(updateHandler))
	r.POST("/logout", sess.HRHandle(logoutHandler))
	r.GET("/refresh", sess.HRHandle(refreshHandler))

	// functions for use by load-test and for debugging and profiling
	r.POST("/addpwdb", plain.HRHandle(addpwdbHandler))
	r.POST("/exit", plain.HRHandle(exitHandler))
	r.GET("/debug", root.HRHandle(debugHandler))
	r.GET("/profcpu", profCPUHandler)
	r.GET("/profmem", profMemHandler)

	log.Fatal(http.ListenAndServe(Config.ListenAndServeAddr, r))
}

// Inform the client whether we're using cookies or tokens.
func authTypeHandler(c *qctx.Ctx) {
	switch qsStore.AuthType {
	case qsess.CookieAuth:
		io.WriteString(c.W, "cookie")
	case qsess.TokenAuth:
		io.WriteString(c.W, "token")
	}
}

func loginHandler(c *qctx.Ctx) {
	if _, _, err := qsStore.GetSession(c.W, c.R); err == nil {
		c.Error("you are already logged in", http.StatusBadRequest)
		return
	}

	var lReq LoginRequest
	if !c.ReadJSON(&lReq) {
		return
	}
	userID, err := pwAuth.auth(lReq.Username, lReq.Password)
	if err != nil {
		c.Error("username/password incorrect", http.StatusUnauthorized)
		return
	}

	c.Sess = qsStore.NewSession(userID)

	sd := c.Sess.Data.(*mySessData)
	sd.userid = userID
	sd.username = lReq.Username
	sd.note = "(nothing)"

	if err := c.Sess.Save(c.W); err != nil {
		c.Error(err.Error(), http.StatusInternalServerError)
		return
	}

	switch qsStore.AuthType {
	case qsess.TokenAuth:
		token, ttl, err := c.Sess.Token()
		if err != nil {
			c.Sess.Delete(c.W)
			c.Error("token creation failed", http.StatusInternalServerError)
			return
		}
		c.WriteJSON(&TokenResponse{Token: token, TimeToLiveSecs: ttl})
	case qsess.CookieAuth:
		// Save has already put cookie into response header. Now just send TTL.
		c.WriteJSON(&TTLResponse{TimeToLiveSecs: c.Sess.MaxAgeSecs})
	}
}

func logoutHandler(c *qctx.Ctx) {
	if err := c.Sess.Delete(c.W); err != nil {
		c.Error(err.Error(), http.StatusInternalServerError)
		return
	}
}

// Fetch session data.
func fetchHandler(c *qctx.Ctx) {
	sd := c.Sess.Data.(*mySessData)
	c.WriteJSON(&FetchResponse{Username: html.EscapeString(sd.username), Note: html.EscapeString(sd.note)})
}

// Update session data.
func updateHandler(c *qctx.Ctx) {
	var uReq UpdateRequest
	if !c.ReadJSON(&uReq) {
		return
	}
	if utf8.RuneCountInString(uReq.Note) > 80 {
		c.Error("note may not be longer than 80 characters", http.StatusBadRequest)
		return
	}

	c.Sess.Data.(*mySessData).note = uReq.Note
	if err := c.Sess.Save(c.W); err != nil {
		c.Error(err.Error(), http.StatusInternalServerError)
		return
	}
}

// Reset session expiration time.
func refreshHandler(c *qctx.Ctx) {
	if err := c.Sess.Save(c.W); err != nil {
		c.Error(err.Error(), http.StatusInternalServerError)
		return
	}

	if qsStore.AuthType == qsess.TokenAuth {
		token, ttl, err := c.Sess.Token()
		if err != nil {
			c.Error("token creation failed", http.StatusInternalServerError)
			return
		}
		c.WriteJSON(&TokenResponse{Token: token, TimeToLiveSecs: ttl})
	}
}

// addpwdbHandler exists solely to allow the load test to insert entries into
// the password store. It accepts an integer "userid" parameter, which would
// not be generated by, or visible to, users in a production app.
func addpwdbHandler(c *qctx.Ctx) {
	var aReq AddpwdbRequest
	if !c.ReadJSON(&aReq) {
		return
	}
	if err := pwAdd.add(aReq.Username, aReq.Password, aReq.Userid); err != nil {
		c.Error("failed to add password database entry", http.StatusInternalServerError)
		return
	}
}

// Shut down this server by calling os.Exit, after allowing time for
// the response to this request to be transmitted to the client.
func exitHandler(c *qctx.Ctx) {
	log.Println("shutting down...")
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}

// Place-holder for temporary debugging code
func debugHandler(c *qctx.Ctx) {
}

func profCPUHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	pprof.Profile(w, r)
}

func profMemHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	pprof.Handler("heap").ServeHTTP(w, r)
}
