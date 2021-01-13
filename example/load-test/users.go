// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"runtime"
	"sync"
	"time"

	. "github.com/gkong/go-qweb/example/api"
)

const (
	// min/max sizes for random strings
	minNamePass = 6  // if this changes, must delete and regenerate the password table
	maxNamePass = 14 // if this changes, must delete and regenerate the password table
	minNote     = 2
	maxNote     = 60
)

var (
	users []user

	activeUsers      int
	activeUsersMutex sync.Mutex

	avgWaitSecs int
	maxIdle     int // max idle connections for http.Transport
)

type user struct {
	id       uint
	username string
	password string
	jar      *cookiejar.Jar
	client   *http.Client
	token    string
	stop     chan bool // simulation is over, time for user goroutine to exit
	done     bool      // user has noticed the simulation is over and has ceased activity
	// statistical variables:
	// we avoid mutexes, by keeping monotonically increasing totals,
	// written by a single goroutine, and one previous value,
	// which is only read and written by the main goroutine.
	reqCount    uint
	reqPrev     uint
	loginCount  uint
	loginPrev   uint
	logoutCount uint
	logoutPrev  uint
	waitTotal   time.Duration
	waitPrev    time.Duration
}

// NOTE: userInit can take a long time, if it needs to generate and store
// passwords for many users, because a deliberately slow hash is used.

func userInit() {
	rand.Seed(1) // always the same, so multiple runs can share passwords

	var nameBuf [maxNamePass]byte
	var passBuf [maxNamePass]byte

	// to re-use connections as much as possible, use a single Transport
	ht := &http.Transport{
		MaxIdleConnsPerHost: maxIdle,
		// XXX - these times are just copied from http.DefaultTransport
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
	}

	for i := range users {
		users[i].id = uint(i)
		users[i].jar, _ = cookiejar.New(nil)
		users[i].client = &http.Client{
			Transport: ht,
			Jar:       users[i].jar,
			Timeout:   30 * time.Second, // XXX - i think uninitialized (zero) means NO timeout
		}
		users[i].stop = make(chan bool, 1)

		// generate pseudo-random username and password
		name := nameBuf[0 : minNamePass+rand.Intn(maxNamePass-minNamePass+1)]
		users[i].username = randString(name)

		pass := passBuf[0 : minNamePass+rand.Intn(maxNamePass-minNamePass+1)]
		users[i].password = randString(pass)
	}

	// attempt to log in the LAST user. if can't, make passwd entries for ALL users.
	// XXX - could speed up by using bisection to find how many are already in the db.

	u := &users[scenario.NumUsers-1]
	loginJSON, _ := json.Marshal(LoginRequest{Username: u.username, Password: u.password})
	tData, code := upost(u, Config.RootURL+"login", "application/json;charset=UTF-8",
		bytes.NewReader(loginJSON), "make-passwords login", false)
	if code == http.StatusOK {
		// login succeeded; don't need to make any passwords; just log out
		if authType == "token" {
			// must put token into u, or upost(logout) will fail with a 401
			var tResp TokenResponse
			if err := json.Unmarshal(tData, &tResp); err != nil {
				log.Fatal("bad response to make-passwords login")
			}
			u.token = tResp.Token
		}
		upost(u, Config.RootURL+"logout", "application/json;charset=UTF-8", nil,
			"logout after make-passwords check", true)
	} else {
		// insert all passwords into the password store
		for i, u := range users {
			addpwdbJSON, _ := json.Marshal(AddpwdbRequest{Username: u.username, Password: u.password, Userid: i})
			upost(&u, Config.RootURL+"addpwdb", "application/json;charset=UTF-8",
				bytes.NewReader(addpwdbJSON), "adding a password", true)
		}
	}
}

func loginAllUsers() {
	done := make(chan int)

	// to parallelize across a few goroutines - log in every Nth user
	loginAllModulo := func(n, me int) {
		var lReq LoginRequest
		var loginJSON []byte
		for i := range users {
			if (i % n) == me {
				lReq.Username = users[i].username
				lReq.Password = users[i].password
				loginJSON, _ = json.Marshal(lReq)
				tData, _ := upost(&users[i], Config.RootURL+"login", "application/json;charset=UTF-8",
					bytes.NewReader(loginJSON), "alwaysOn initial login", true)
				if authType == "token" {
					var tResp TokenResponse
					if err := json.Unmarshal(tData, &tResp); err != nil {
						log.Fatalf("user %d - bad response to login", users[i].id)
					}
					users[i].token = tResp.Token
				}
				// don't count logins in statistics when in alwaysOn mode
				users[i].reqCount = 0
				users[i].waitTotal = 0
			}
		}
		done <- me
	}

	threads := runtime.GOMAXPROCS(0)
	for i := 0; i < threads; i++ {
		go loginAllModulo(threads, i)
	}
	for i := 0; i < threads; i++ {
		_ = <-done
	}
}

// doUser() simulates one user forever
func doUser(u *user) {
	var noteBuf [maxNote]byte
	loginJSON, _ := json.Marshal(LoginRequest{Username: u.username, Password: u.password})
	updateReq := UpdateRequest{Note: "(nothing)"} // current Note. fetch should compare against this.
	fetchResp := FetchResponse{}

	// user alternates between periods of activity and rest.
	// main loop spins once per active period.
MainLoop:
	for true {
		var endTime time.Time

		if !scenario.AlwaysOn {
			// wait for a random period based on total daily active/rest times
			select {
			case <-u.stop:
				userShutdown(u)
				return
			case <-time.After(time.Duration(uniform2X(avgWaitSecs)) * time.Second):
			}

			endTime = time.Now().Add(time.Duration(uniform2X(scenario.SecsPerActivity)) * time.Second)

			// increment global activeUsers or abort if too many users
			abort := false
			activeUsersMutex.Lock()
			if activeUsers < Config.MaxActive {
				activeUsers++
			} else {
				abort = true
			}
			activeUsersMutex.Unlock()
			if abort {
				continue MainLoop
			}

			// fetch index.html
			if rand.Intn(3) == 0 { // XXX - assume cached 2/3 of the time; should model browser caching
				uget(u, Config.RootURL, "initial fetch of /", true)
				time.Sleep(20 * time.Millisecond) // time from page load to first call to REST API
			}

			// initial fetch, may require login
			data, code := uget(u, Config.RootURL+"fetch", "initial fetch", false)
			switch code {
			case http.StatusOK:
				// already logged in. save note.
				if err := json.Unmarshal(data, &fetchResp); err != nil {
					log.Fatalf("user %d - initial fetch - json unmarshal failed - %s", u.id, err.Error())
				}
				if fetchResp.Username != u.username {
					log.Fatalf("user %d - initial fetch - expect username %s got %s",
						u.id, u.username, fetchResp.Username)
				}
				updateReq.Note = fetchResp.Note
			case http.StatusUnauthorized:
				// not logged in, log in now
				u.loginCount++
				tData, _ := upost(u, Config.RootURL+"login", "application/json;charset=UTF-8",
					bytes.NewReader(loginJSON), "login at start of activity", true)
				if authType == "token" {
					var tResp TokenResponse
					if err := json.Unmarshal(tData, &tResp); err != nil {
						log.Fatalf("user %d - bad response to login", u.id)
					}
					u.token = tResp.Token
				}
				updateReq.Note = "(nothing)"
			default:
				log.Fatalf("initial fetch returned %d", code)
			}
		}

		// inner loop - wait a little while, do a fetch or a update, repeat
		var fetchData []byte // XXX - does it help to declare these outside of the inner loop?
		var err error
		var newNote []byte
		for scenario.AlwaysOn || time.Now().Before(endTime) {
			select {
			case <-u.stop:
				userShutdown(u)
				return
			case <-time.After(time.Duration(uniform2X(scenario.MsBetwReq)) * time.Millisecond):
			}

			switch rand.Intn(2) {
			case 0:
				// call fetch and check all returned data
				fetchData, _ = uget(u, Config.RootURL+"fetch", "fetch during an activity", true)
				if err = json.Unmarshal(fetchData, &fetchResp); err != nil {
					log.Fatalf("user %d - fetch during an activity - json unmarshal failed - %s",
						u.id, err.Error())
				}
				if fetchResp.Username != u.username {
					log.Fatalf("user %d - fetch during an activity - expect username %s got %s",
						u.id, u.username, fetchResp.Username)
				}
				if fetchResp.Note != updateReq.Note {
					log.Fatalf("user %d - fetch during an activity - note mismatch", u.id)
				}
			case 1:
				// generate random data and call update
				newNote = noteBuf[0 : minNote+rand.Intn(maxNote-minNote+1)]
				updateReq.Note = randString(newNote)
				updateJSON, _ := json.Marshal(updateReq)
				upost(u, Config.RootURL+"update", "application/json;charset=UTF-8",
					bytes.NewReader(updateJSON), "update during an activity", true)
			}
		}

		if rand.Float64() < scenario.FracLogout {
			u.logoutCount++
			upost(u, Config.RootURL+"logout", "application/json;charset=UTF-8",
				nil, "logout", true)
		}

		if !scenario.AlwaysOn {
			activeUsersMutex.Lock()
			activeUsers--
			activeUsersMutex.Unlock()
		}
	}
}

func userShutdown(u *user) {
	upost(u, Config.RootURL+"logout", "application/json;charset=UTF-8", nil,
		"shutdown logout", false)
	u.done = true
}
