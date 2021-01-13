// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main_test

import (
	"bytes"
	"log"
	"os/exec"
	"testing"
	"time"
)

// run each database back-end at least once.
// exercise both cookies and tokens.
// try the most likely combinations: goleveldb/goleveldb and cassandra/cassandra.
var paramsList = [][]string{
	{"-sessiondb=map", "-passworddb=goleveldb", "-qsess-authtype=cookie"},
	{"-sessiondb=goleveldb", "-passworddb=goleveldb", "-qsess-authtype=token"},
	{"-sessiondb=cassandra", "-passworddb=cassandra", "-qsess-authtype=token"},
	{"-sessiondb=mysql", "-passworddb=cassandra", "-qsess-authtype=cookie"},
}

var serverName = "./server-full"
var clientName = "./load-test"

func TestLoad(t *testing.T) {
	log.Printf("This should take several minutes...\n")

	for _, params := range paramsList {
		log.Printf("%s %s %s %s\n", serverName, params[0], params[1], params[2])

		// start the server
		serverCmd := exec.Command(serverName, params...)
		var serverOut bytes.Buffer
		serverCmd.Stdout = &serverOut
		serverCmd.Stderr = &serverOut
		if err := serverCmd.Start(); err != nil {
			t.Errorf("%s start failure - %s\n", serverName, err.Error())
		} else {
			t.Logf("started %s %s %s %s\n", serverName, params[0], params[1], params[2])
		}

		time.Sleep(2 * time.Second)

		// run the client
		t.Logf("starting %s -scenario=test -serverexit=true", clientName)
		ltCmd := exec.Command(clientName, "-scenario=test", "-serverexit=true")
		ltCmd.Dir = "../load-test"
		var ltOut bytes.Buffer
		ltCmd.Stdout = &ltOut
		ltCmd.Stderr = &ltOut
		if err := ltCmd.Run(); err != nil {
			t.Errorf("%s returned error - %s\n", clientName, err.Error())
		}

		// wait for server to exit. shouldn't take more than a few seconds,
		// since client has already issued a server exit request.
		cancel := make(chan int, 1)
		killed := false
		go func() {
			select {
			case <-cancel:
				return
			case <-time.After(15 * time.Second):
				killed = true
				serverCmd.Process.Kill()
			}
		}()
		if err := serverCmd.Wait(); err != nil {
			t.Errorf("%s exited with error - %s\n", serverName, err.Error())
		}
		cancel <- 1

		t.Log("server:")
		t.Log(serverOut.String())
		t.Log("load-test:")
		t.Log(ltOut.String())

		if killed {
			t.Fatalf("%s never exited, needed to be killed.", serverName)
		}
	}
}
