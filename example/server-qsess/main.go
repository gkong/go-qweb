// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Simple restful back end, based only on qsess.
//
// It supports a client consisting of a login page and a home page,
// which allows users to view and modify a note maintained in session storage.
//
// Run it and visit: http://localhost:8080
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"unicode/utf8"

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

	// static files - js, css, etc.
	http.Handle("/static/", http.FileServer(http.Dir("../../example/client")))

	// restful API endpoints
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/fetch", fetchHandler)
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/logout", logoutHandler)

	// We're a single-page app, so all URLs that are not static files or
	// API endpoints return index.html and let the front end decide what to do.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../example/client/index.html")
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if _, _, err := qsStore.GetSession(w, r); err == nil {
		http.Error(w, "you are already logged in", http.StatusBadRequest)
		return
	}

	var params struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "bad params", http.StatusBadRequest)
		return
	}

	// dummy authentication code - only accept user "me" and password "abc"
	if params.Username != "me" || params.Password != "abc" {
		http.Error(w, "username/password incorrect", http.StatusUnauthorized)
		return
	}

	s := qsStore.NewSession(nil)
	sd := s.Data.(*MySessData)
	sd.Userid = 0
	sd.Username = params.Username
	sd.Note = "(nothing)"

	if err := s.Save(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var response struct {
		TimeToLiveSecs int `json:"ttl"`
	}
	response.TimeToLiveSecs = s.MaxAgeSecs
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	sess, _, err := qsStore.GetSession(w, r)
	if err != nil {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	if err = sess.Delete(w); err != nil {
		http.Error(w, "cannot log out", http.StatusInternalServerError)
		return
	}
}

// Fetch session data.
func fetchHandler(w http.ResponseWriter, r *http.Request) {
	sess, ttl, err := qsStore.GetSession(w, r)
	if err != nil {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	if ttl < (sess.MaxAgeSecs - sess.MinRefreshSecs) {
		sess.Save(w) // refresh
	}

	sd := sess.Data.(*MySessData)
	w.Header().Set("Content-Type", "application/json")

	var response struct {
		Username string `json:"username"`
		Note     string `json:"note"`
	}
	response.Username = sd.Username
	response.Note = sd.Note
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Update session data.
func updateHandler(w http.ResponseWriter, r *http.Request) {
	sess, _, err := qsStore.GetSession(w, r)
	if err != nil {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	var params struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "bad params", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(params.Note) > 80 {
		http.Error(w, "note may not be longer than 80 characters", http.StatusBadRequest)
		return
	}

	sess.Data.(*MySessData).Note = params.Note
	sess.Save(w)
}
