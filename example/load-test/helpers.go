// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// http get/post helpers

func uget(u *user, url string, errMsgPrefix string, mustBeOK bool) ([]byte, int) {
	var t time.Time
	var resp *http.Response
	var err error

	u.reqCount++

	if scenario.ShowWait {
		t = time.Now()
	}

	if authType == "token" && u.token != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatalf("uget: user %d - %s - NewRequest failed - %s", u.id, errMsgPrefix, err.Error())
		}
		req.Header.Add("Authorization", "Bearer "+u.token)
		resp, err = u.client.Do(req)
	} else {
		resp, err = u.client.Get(url)
	}

	if scenario.ShowWait {
		u.waitTotal += time.Now().Sub(t)
	}

	if err != nil {
		log.Fatalf("uget: user %d - %s - Get failed - %s", u.id, errMsgPrefix, err.Error())
	}

	// must read to EOF or connection will not be reused
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close() // XXX - need this?

	if mustBeOK && (resp.StatusCode != http.StatusOK) {
		log.Fatalf("uget: user %d - %s - status code %d, response body = %s",
			u.id, errMsgPrefix, resp.StatusCode, string(b))
	} else if err != nil {
		log.Fatalf("uget: user %d - %s - response body read failure - %s", u.id, errMsgPrefix, err.Error())
	}

	return b, resp.StatusCode
}

func upost(u *user, url string, contentType string, body io.Reader, errMsgPrefix string, mustBeOK bool) ([]byte, int) {
	var t time.Time
	var resp *http.Response
	var err error

	u.reqCount++

	if scenario.ShowWait {
		t = time.Now()
	}

	if authType == "token" && u.token != "" {
		req, err := http.NewRequest("POST", url, body)
		if err != nil {
			log.Fatalf("upost: user %d - %s - NewRequest failed - %s", u.id, errMsgPrefix, err.Error())
		}
		req.Header.Add("Content-type", contentType)
		req.Header.Add("Authorization", "Bearer "+u.token)
		resp, err = u.client.Do(req)
	} else {
		resp, err = u.client.Post(url, contentType, body)
	}

	if scenario.ShowWait {
		u.waitTotal += time.Now().Sub(t)
	}

	if err != nil {
		log.Fatalf("upost: user %d - %s - Post failed - %s", u.id, errMsgPrefix, err.Error())
	}

	// must read to EOF or connection will not be reused
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close() // XXX - need this?
	if mustBeOK && (resp.StatusCode != http.StatusOK) {
		log.Fatalf("upost: user %d - %s - status code %d, response body = %s",
			u.id, errMsgPrefix, resp.StatusCode, string(b))
	} else if err != nil {
		log.Fatalf("upost: user %d - %s - response body read failure - %s", u.id, errMsgPrefix, err.Error())
	}

	return b, resp.StatusCode
}

func secsToString(secs uint) string {
	return fmt.Sprintf("%2.2d:%2.2d:%2.2d", secs/(60*60), (secs/60)%60, secs%60)
}

// uniform distribution from zero to 2*mean.
func uniform2X(mean int) int {
	return rand.Intn(2 * mean)
}

// fill in a byte slice with random letters
func randString(b []byte) string {
	letters := [...]byte{
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
		'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
		'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
	}

	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// convert uint64 to []byte
func uitob(b []byte, u uint64) {
	binary.LittleEndian.PutUint64(b, u)
}
