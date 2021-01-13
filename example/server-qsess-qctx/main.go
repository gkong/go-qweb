// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Simple restful back end, based on qsess and qctx.
//
// It supports a client consisting of a login page and a home page,
// which allows users to view and modify a note maintained in session storage.
//
// Run it and visit: http://localhost:8080
package main

import (
	"html"
	"log"
	"net/http"
	"os"
	"unicode/utf8"

	. "github.com/gkong/go-qweb/example/api"
	"github.com/gkong/go-qweb/qctx"
	"github.com/gkong/go-qweb/qsess"
)

var qsStore *qsess.Store

// MySessData is an example custom session data type implementation.
// github.com/tinylib/msgp was used to generate a serializer/deserializer.

//go:generate msgp -o=serializer_generated.go -tests=false

type MySessData struct {
	Userid   int64
	Username string
	Note     string
}

func newMySessData() qsess.SessData {
	return &MySessData{}
}

func (m *MySessData) Marshal() ([]byte, error) {
	return m.MarshalMsg([]byte{})
}

func (m *MySessData) Unmarshal(b []byte) error {
	_, err := m.UnmarshalMsg(b)
	return err
}

func main() {
	var err error

	qsStore, err = qsess.NewMapStore([]byte("encryption-key------------------"))
	if err != nil {
		log.Fatalln("NewMapStore failed - " + err.Error())
	}
	qsStore.NewSessData = newMySessData

	// middleware stacks
	root := qctx.MwStack(qctx.MwRecovery(os.Stderr, true, false), qctx.MwLogger(os.Stderr, true))
	sess := root.Append(qctx.MwRequireSess(qsStore))

	// static files - js, css, etc.
	http.Handle("/static/", http.FileServer(http.Dir("../../example/client")))

	// restful API endpoints
	http.Handle("/login", root.Handle(loginHandler))
	http.Handle("/fetch", sess.Handle(fetchHandler))
	http.Handle("/update", sess.Handle(updateHandler))
	http.Handle("/logout", sess.Handle(logoutHandler))

	// We're a single-page app, so all URLs that are not static files or
	// API endpoints return index.html and let front end decide what to do.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "../../example/client/index.html") })

	log.Fatal(http.ListenAndServe(":8080", nil))
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

	// dummy authentication code - only accept user "me" and password "abc"
	if lReq.Username != "me" || lReq.Password != "abc" {
		c.Error("username/password incorrect", http.StatusUnauthorized)
		return
	}

	s := qsStore.NewSession(nil)
	sd := s.Data.(*MySessData)
	sd.Userid = 0
	sd.Username = lReq.Username
	sd.Note = "(nothing)"

	if err := s.Save(c.W); err != nil {
		c.Error(err.Error(), http.StatusInternalServerError)
		return
	}

	c.WriteJSON(&TTLResponse{TimeToLiveSecs: s.MaxAgeSecs})
}

func logoutHandler(c *qctx.Ctx) {
	if err := c.Sess.Delete(c.W); err != nil {
		c.Error("cannot log out", http.StatusInternalServerError)
		return
	}
}

// Fetch session data.
func fetchHandler(c *qctx.Ctx) {
	sd := c.Sess.Data.(*MySessData)
	c.WriteJSON(&FetchResponse{Username: html.EscapeString(sd.Username), Note: html.EscapeString(sd.Note)})
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

	c.Sess.Data.(*MySessData).Note = uReq.Note
	c.Sess.Save(c.W)
}
