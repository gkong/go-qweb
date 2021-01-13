// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"log"
	"os"

	"github.com/gkong/go-qweb/qsess"
	"github.com/gkong/go-qweb/qsess/qscql"
	"github.com/gkong/go-qweb/qsess/qsldb"
	"github.com/gkong/go-qweb/qsess/qsmy"
)

var qsStore *qsess.Store

// set up qsStore to contain a store of the specified database type
func sessSetup(dbt dbTypeEnum) {
	dbSetup(dbt)

	switch dbt {
	case goleveldbDB:
		gldbSessStore()
	case cassandraDB:
		cassandraSessStore()
	case mysqlDB:
		mysqlSessStore()
	case mapDB:
		mapSessStore()
	}
}

func sessParams() {
	// doConfig has validated authtype, so we can ignore error return
	qsStore.AuthType, _ = qsess.ParseAuthType(Config.Qsess.AuthType)

	qsStore.MaxAgeSecs = Config.Qsess.MaxAgeSecs
	qsStore.MinRefreshSecs = Config.Qsess.MinRefreshSecs
	qsStore.CookieSecure = Config.Qsess.CookieSecure
	qsStore.CookieHTTPOnly = Config.Qsess.CookieHTTPOnly

	qsStore.NewSessData = newMySessData
}

func mapSessStore() {
	var err error

	qsStore, err = qsess.NewMapStore([]byte(Config.Qsess.EncryptKey))
	if err != nil {
		log.Fatalln("mapSessSetup - NewCqlStore failed - " + err.Error())
	}

	sessParams()
}

func gldbSessStore() {
	var err error

	qsStore, err = qsldb.NewGldbStore(gldb, istobs(Config.Goleveldb.SessKeyPrefix),
		os.Stderr, []byte(Config.Qsess.EncryptKey))
	if err != nil {
		log.Fatalln("gldbSessSetup - NewGldbStore failed - " + err.Error())
	}

	sessParams()
}

func cassandraSessStore() {
	var err error

	qsStore, err = qscql.NewCqlStore(cdb, Config.Cassandra.SessTableName,
		Config.Cassandra.SessUIDIndex, Config.Cassandra.SessUIDToClient,
		[]byte(Config.Qsess.EncryptKey))
	if err != nil {
		log.Fatalln("cqlSessSetup - NewCqlStore failed - " + err.Error())
	}

	sessParams()
}

func mysqlSessStore() {
	var err error

	qsStore, err = qsmy.NewMysqlStore(sdb, Config.Mysql.SessTableName,
		Config.Mysql.SessDataColDef, Config.Mysql.SessUIDColDef,
		[]byte(Config.Qsess.EncryptKey))
	if err != nil {
		log.Fatalln("sqlSessSetup - NewMysqlStore failed - " + err.Error())
	}

	sessParams()
}
