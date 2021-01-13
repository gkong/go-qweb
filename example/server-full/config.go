// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"log"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gocql/gocql"
	"github.com/koding/multiconfig"
)

const configFile = "./config.toml"

var Config struct {
	SessionDB          string `required:"true"`
	PasswordDB         string `required:"true"`
	ListenAndServeAddr string `required:"true"`
	BcryptCost         int    `required:"true"`
	Logging            bool

	Qsess     ConfigQsess `required:"true"`
	Goleveldb ConfigGldb
	Mysql     ConfigMysql
	Cassandra ConfigCassandra
}

type ConfigQsess struct {
	AuthType       string `required:"true"`
	MaxAgeSecs     int    `required:"true"`
	MinRefreshSecs int
	CookieSecure   bool
	CookieHTTPOnly bool
	EncryptKey     string `required:"true"`
}

type ConfigGldb struct {
	DbFile          string `required:"true"`
	SessKeyPrefix   []int
	PasswdKeyPrefix []int
}

type ConfigMysql struct {
	User           string `required:"true"`
	Password       string `required:"true"`
	Database       string `required:"true"`
	SessTableName  string // required if using for session store
	SessDataColDef string // required if using for session store
	SessUIDColDef  string // required if using for session store
	MaxOpenConns   int
	MaxIdleConns   int
}

type ConfigCassandra struct {
	ClusterAddr       string `required:"true"`
	KeySpace          string `required:"true"`
	ProtoVersion      int    `required:"true"`
	Consistency       string `required:"true"`
	SessUIDIndex      bool
	SessUIDToClient   bool
	SessTableName     string // required if using for session store
	PasswordTableName string // required if using for password table
	TimeoutMsec       int
}

func doConfig() {
	m := multiconfig.NewWithPath(configFile)
	m.MustLoad(&Config)

	// custom enum types - tried to use structs with UnmarshalText() methods,
	// but they don't work as command-line args or environment variables,
	// so just declare them as strings and validate here.
	// Fortunately, they happen to be used only during initial setup,
	// so they don't have to be parsed repeatedly or stored redundantly.
	sessDB, err := parseDbType(Config.SessionDB)
	if err != nil {
		panic("bad SessionDB")
	}
	passDB, err := parseDbType(Config.PasswordDB)
	if err != nil {
		panic("bad PasswordDB")
	}
	authType, err := qsess.ParseAuthType(Config.Qsess.AuthType)
	if err != nil {
		panic("bad Qsess.AuthType")
	}
	if sessDB == cassandraDB || passDB == cassandraDB {
		// if this doesn't panic, we have a valid consistency string
		gocql.ParseConsistency(Config.Cassandra.Consistency)
	}

	sessSetup(sessDB)
	pwSetup(passDB)

	log.Printf("SessionDB = %s, PasswordDB = %s, SessAuthType = %s\n", sessDB, passDB, authType)
}

// convert []int to []byte
func istobs(is []int) []byte {
	bs := make([]byte, len(is))
	for index, value := range is {
		bs[index] = byte(value)
	}
	return bs
}
