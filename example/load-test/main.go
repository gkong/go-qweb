// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Generate HTTP requests, simulating lots of users. This is not intended
// to be a general-purpose load generator; it is tightly coupled with
// its sister program, "app," which is a sample RESTful back-end.
//
// At startup: make one goroutine per simulated user.
// User goroutines live forever, until the simulation is terminated, either by
// completing testSecs or by receipt of a SIGINT (e.g. user typing ctl-c).
//
// config.toml includes many tunable parameters.
// Users can alternate between activity and rest (see the alwaysOn flag).
// Various interesting combinations of parameters are included in comments.
// The default set of parameters define a quick test with light load.
//
// Users are created with random usernames and passwords. If they are not in
// the password table, they are added, so the server can authenticate them.
// The same random seed is always used, so passwords are the same across multiple runs.
// This program does not read or write the password table during on-going operation,
// just at start-up time, to ensure that all its users are in the password
// table, so the restful back-end can autheticate them.
//
// Note that, if numUsers is large, it could take hours to create the password
// database, since the server calls bcrypt (intentionally slow) to hash each password.
package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
)

const ivlSecs = 1

var authType string // "cookies" or "tokens". we get this from the server.

var cpuProfFile *os.File
var memProfFile *os.File

func profileInit() {
	var err error
	if Config.CpuProfFile != "" {
		cpuProfFile, err = os.Create(Config.CpuProfFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	if Config.MemProfFile != "" {
		memProfFile, err = os.Create(Config.MemProfFile)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func profileBegin() {
	if Config.CpuProfFile != "" {
		pprof.StartCPUProfile(cpuProfFile)
	}
}

func profileEnd() {
	if Config.CpuProfFile != "" {
		pprof.StopCPUProfile()
	}
	if Config.MemProfFile != "" {
		pprof.WriteHeapProfile(memProfFile)
	}
}

func main() {
	doConfig()
	profileInit()
	getAuthType() // must call this before userInit
	userInit()

	if scenario.AlwaysOn {
		loginAllUsers()
	}

	// launch one goroutine per simulated user.
	for i := range users {
		go doUser(&users[i])
	}

	profileBegin()

	// main thread now loops forever, calculating and printing statistics

	timesUp := time.After(time.Duration(scenario.TestSecs) * time.Second)

	ctlC := make(chan os.Signal, 1)
	signal.Notify(ctlC, os.Interrupt)

	prevTime := time.Now()

	for {
		select {
		case <-timesUp:
			shutDown()
			return
		case <-ctlC:
			shutDown()
			return
		default:
		}

		time.Sleep(prevTime.Add(ivlSecs * time.Second).Sub(time.Now()))
		prevTime = time.Now()

		if scenario.ShowStats {
			stats()
		}
	}
}

func shutDown() {
	profileEnd()

	log.Println("shutting down (may take several seconds)...")

	for i := range users {
		users[i].stop <- true
	}

	var stillActive uint

	for secs := 0; secs < 60; secs++ {
		time.Sleep(1 * time.Second)
		stillActive = 0
		for i := range users {
			if !users[i].done {
				stillActive++
			}
		}
		if stillActive > 0 {
			log.Printf("%d active\n", stillActive)
		} else {
			break
		}
	}

	if stillActive > 0 {
		log.Printf("exiting with %d still active\n", stillActive)
	} else {
		log.Println("all inactive, exiting.")
	}

	if Config.ServerExit {
		upost(&users[0], Config.RootURL+"exit", "text/plain; charset=utf-8", nil, "exit", true)
	}
}

// ask server if it's using cookies or tokens, save answer in global authType
func getAuthType() {
	resp, err := http.Get(Config.RootURL + "authtype")
	if err != nil {
		log.Fatalln("GET /authtype failed")
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Fatalln("problem with response to GET /authtype")
	}
	authType = string(b)
	if (authType != "cookie") && (authType != "token") {
		log.Fatalln("bad authType - " + authType)
	}
}
