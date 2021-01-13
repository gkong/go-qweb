// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"fmt"
	"time"
)

const (
	ewmaWeight  = 0.05 // for calculating exponentially-weighted moving averages
	startupIvls = 4    // stats intervals to skip at startup, must be at least 3
)

var activeEWMA float64
var reqsEWMA float64
var loginsEWMA float64
var logoutsEWMA float64
var waitEWMA float64

var activeMax int
var reqsMax float64
var loginsMax float64

var reqsAvgMax float64

var statsCount = 0 // how many times have we been called?
var statsStartTime time.Time
var statsPrevTime time.Time

// called periodically, to gather and print statistics

func stats() {
	statsCount++
	if statsCount == 1 {
		// first time thru - just do some initializations
		statsStartTime = time.Now()
		statsPrevTime = statsStartTime
		return
	}

	now := time.Now()
	secsSinceStart := uint(time.Since(statsStartTime).Seconds())
	deltaT := now.Sub(statsPrevTime)
	statsPrevTime = now

	var activeNow = activeUsers

	var reqsNow uint
	var loginsNow uint
	var logoutsNow uint
	var waitNow = time.Duration(0)

	// calculate things that must be summed over all users
	for i := range users {
		reqs := users[i].reqCount
		reqsNow += reqs - users[i].reqPrev
		users[i].reqPrev = reqs

		logins := users[i].loginCount
		loginsNow += logins - users[i].loginPrev
		users[i].loginPrev = logins

		logouts := users[i].logoutCount
		logoutsNow += logouts - users[i].logoutPrev
		users[i].logoutPrev = logouts

		if scenario.ShowWait {
			wait := users[i].waitTotal
			waitNow += wait - users[i].waitPrev
			users[i].waitPrev = wait
		}
	}

	if activeNow > activeMax {
		activeMax = activeNow
	}

	reqsPerSec := float64(reqsNow) / deltaT.Seconds()
	if reqsPerSec > reqsMax {
		reqsMax = reqsPerSec
	}

	loginsPerSec := float64(loginsNow) / deltaT.Seconds()
	if loginsPerSec > loginsMax {
		loginsMax = loginsPerSec
	}

	logoutsPerSec := float64(logoutsNow) / deltaT.Seconds()

	activeEWMA = (float64(activeNow) * ewmaWeight) + (activeEWMA * (1.0 - ewmaWeight))
	reqsEWMA = (reqsPerSec * ewmaWeight) + (reqsEWMA * (1.0 - ewmaWeight))
	loginsEWMA = (loginsPerSec * ewmaWeight) + (loginsEWMA * (1.0 - ewmaWeight))
	logoutsEWMA = (logoutsPerSec * ewmaWeight) + (logoutsEWMA * (1.0 - ewmaWeight))

	if reqsEWMA > reqsAvgMax {
		reqsAvgMax = reqsEWMA
	}

	var waitMsPerReq float64
	if scenario.ShowWait {
		waitMsPerReq = float64(waitNow.Nanoseconds()) / float64(1000*reqsNow)
		waitEWMA = (waitMsPerReq * ewmaWeight) + (waitEWMA * (1.0 - ewmaWeight))
	}

	if statsCount <= startupIvls {
		// to minimize start-up distortions, set EWMAs to current instantaneous values and maxes to zero
		activeEWMA = float64(activeNow)
		reqsEWMA = reqsPerSec
		loginsEWMA = float64(loginsNow)
		logoutsEWMA = float64(logoutsNow)
		waitEWMA = waitMsPerReq
		activeMax = 0
		reqsMax = 0.0
		loginsMax = 0.0
		reqsAvgMax = 0.0
	} else {
		// print everything we've just calculated
		if scenario.AlwaysOn {
			fmt.Printf("%d active", scenario.NumUsers)
		} else {
			fmt.Printf("%d active (%d avg, %d max)", activeNow, uint(activeEWMA+0.5), activeMax)
		}
		fmt.Printf(", %d req/sec (%d avg, %d avg max, %d max)",
			uint(reqsPerSec+0.5), uint(reqsEWMA+0.5), uint(reqsAvgMax+0.5), uint(reqsMax+0.5))
		if !scenario.AlwaysOn {
			fmt.Printf(", %d login/s (%d avg, %d max), %d logout/s (%d avg)",
				uint(loginsPerSec+0.5), uint(loginsEWMA+0.5), uint(loginsMax+0.5),
				uint(logoutsPerSec+0.5), uint(logoutsEWMA+0.5),
			)
		}
		if scenario.ShowWait {
			fmt.Printf(", %.2f usec/req (%.2f avg)", waitMsPerReq, waitEWMA)
		}
		fmt.Printf(", %s, %.2f\n", secsToString(secsSinceStart), deltaT.Seconds())
	}
}
