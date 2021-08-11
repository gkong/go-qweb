[![Go Reference](https://pkg.go.dev/badge/github.com/gkong/go-qweb.svg)](https://pkg.go.dev/github.com/gkong/go-qweb)

# go-qweb - golang web app components

This module contains light-weight, minimal-dependency golang packages useful for building web apps.
It includes a load test program which can generate heavy simulated traffic,
for exercising the components and observing their performance.

# qsess
Package `qsess` is a server-side web session manager.
It manages data which persists for the life of a session,
so that one invocation of a REST API can store data which all subsequent calls
during the life of the session can read and modify.
It also manages client access to sessions via cookies or tokens.

### uses
A login function can authenticate a user and use `qsess`
to store a user ID in session storage and return a cookie or token to the client.
In each subsequent request, the client includes the cookie or token,
and `qsess` checks the validity of the cookie or token
(thereby authenticating the client) and makes the
user ID available to the request handler.

An email address validation function can use `qsess` to create a session containing
an email address and other user information and generate a token to send to the user in an email.
Later, the user clicks a link in the email, causing their browser to present the token to the application,
which retrieves the user's email address and other information from session storage and completes the validation.
If the user waits too long, the session automatically expires, invalidating the token.

### features
- user-definable session data types
- authentication via cookies and tokens
- session expiration
- revocation of all sessions for a given user id, for secure password changes
- back-ends for goleveldb, Cassandra/Scylla, PostgreSQL, MySQL, and a simple, in-memory store

### zero dependencies
`qsess` is independent of, but integrates easily with, routers,
middleware frameworks, http.Request.Context(), etc.
The core package has zero dependencies.
Database back-ends, which reside in sub-packages, each depend only on a database-specific driver module.
Back-end sub-packages currently include
`qscql` (Cassandra/Scylla), `qsldb` (goleveldb), `qspgx` (PostgreSQL), and `qsmy` (MySQL).

# qctx
Package `qctx` is a light-weight, type-safe, per-http-request state manager.

Whereas `qsess` manages data that persists during an entire session,
which can include many HTTP requests over a long time,
`qctx` manages data that only persists for the life of a single HTTP request.
This allows independently-coded software modules, such as stackable
middleware modules, to share data.

The package includes a simple middleware stacking facility and a few middleware and helper functions.

Support is included for `net/http`,
`julienschmidt/httprouter`, and `qsess` sessions.

# example web application and load test
Directory `example` contains a single-page web application,
consisting of a login "page" and a home "page," which displays a plain-text
note (maintained in session storage) and allows you to update the note.

![Example App Screen Shot](home.png?raw=true)

There is a single HTML/JavaScript client, and several versions of a RESTful server.
All versions of the server implement the same basic functions and operate with the same client.

### server-qsess
Directory `server-qsess` contains a simple server, with minimal dependencies.
Its authentication code is a stub, which recognizes only a single, hard-coded user.

### server-qsess-qctx
Directory `server-qsess-qctx` contains a server simlar to `server-qsess`,
simplified through the use of middleware and helpers from package `qctx`.
Its handlers comprise only about half as many lines of code.

### server-full
Directory `server-full` contains a server expanded to support the included load test.
It uses julienschmidt/httprouter, authenticates users against a password database,
and supports both session tokens and session cookies.
It is configurable, through a configuration file and/or command-line parameters,
and it supports load-testing of all of the qsess database back-ends.

### load-test
Directory `load-test` contains a program which simulates many users interacting
with the RESTful API provided by `server-full`. It can generate a steady load,
and it can have individual simulated users randomly switch between periods of
activity and periods of rest with login/logout.

![Load Test Screen Shot](loadtest.png?raw=true)

# Install and Run

	cd example/server-qsess
	go build
	./server-qsess
	# visit with a web browser - http://localhost:8080
	# log in as user 'me' with password 'abc'. simple app just sets and displays a string.
	# when finished, type ctl-C, to kill server-qsess

### run a load test

	# make sure no other server is running on localhost:8080
	cd example/server-full
	go build
	./server-full
	# open another terminal window
	cd example/load-test
	./load-test  # takes a few secs to start, then runs for 1 min, displaying stats
	./load-test -scenario=steady -numusers=2000  # 1 req/user/sec for 24 hrs, ctl-C to stop
	# these tests use only the compiled-in goleveldb database.
	# the first run takes longer, because it has to populate the password database.
	# load tests can fail if system resources are exhausted; see example/load-test/tuning-linux.txt.

### exercise all database back-ends

	# install databases you want to exercise: postgresql, mysql, cassandra (or scyllaDB)
	# goleveldb needs no installation; it's just a go module, compiled into your application
	# setup up postgresql according to instructions in qspgx/pgx_test.go
	# set up mysql according to instructions in qsmy/mysql_test.go
	# to test individual database back-ends, run "go test" in each of these places:
		qsess
		qsess/qscql
		qsess/qsldb
		qsess/qspgx
		qsess/qsmy

	cd example/server-full
	# set up postgresql and mysql according to instructions in example/server-full/dbsetup.go.
	# if using scyllaDB, edit server-full/config.toml and set protoversion to 3.
	# make sure the example server is not running in any other window.
	go build
	go test  # takes about 3 mins, exercises all databases and both cookies and tokens
	# if you see errors, especially with MySQL, see example/load-test/tuning-linux.txt.

### run a longer load test on a specific database

	# set up mysql according to instructions in example/server-full/dbsetup.go
	# view config.toml in server-full and load-test directories for all options
	./server-full -help  # see available command-line args and env vars
	./load-test -help  # see available command-line args and env vars
	./server-full -passworddb=cassandra -sessiondb=cassandra -qsess-authtype=token
	./load-test -scenario=manyusers  # 200K users, takes a couple of hours to start
