// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	gldb *leveldb.DB
	pdb  *pgxpool.Pool
	sdb  *sql.DB
	cdb  *gocql.Session

	noctx = context.Background()
)

type dbTypeEnum int

const (
	mapDB dbTypeEnum = iota
	goleveldbDB
	cassandraDB
	mysqlDB
	postgresqlDB
)

func (dbt dbTypeEnum) String() string {
	switch dbt {
	case goleveldbDB:
		return "goleveldb"
	case cassandraDB:
		return "cassandra"
	case postgresqlDB:
		return "postgresql"
	case mysqlDB:
		return "mysql"
	case mapDB:
		return "map"
	default:
		return "!UNDEFINED-dbTypeEnum!"
	}
}

func parseDbType(s string) (dbt dbTypeEnum, err error) {
	switch strings.ToLower(s) {
	case "map":
		dbt = mapDB
	case "goleveldb":
		dbt = goleveldbDB
	case "cassandra":
		dbt = cassandraDB
	case "postgresql":
		dbt = postgresqlDB
	case "mysql":
		dbt = mysqlDB
	default:
		return 0, errors.New(s + " is not a valid database type")
	}
	return dbt, nil
}

func dbSetup(dbt dbTypeEnum) {
	switch dbt {
	case goleveldbDB:
		gldbSetup()
	case cassandraDB:
		cassandraSetup()
	case postgresqlDB:
		postgresqlSetup()
	case mysqlDB:
		mysqlSetup()
	case mapDB:
		// no setup needed
	}
}

func dbClose() {
	if gldb != nil {
		gldb.Close()
	}
	if sdb != nil {
		sdb.Close()
	}
	if cdb != nil {
		cdb.Close()
	}
	if pdb != nil {
		pdb.Close()
	}
}

// goleveldb - database code is linked into application binary, no separate server

func gldbSetup() {
	var err error

	if gldb != nil {
		return
	}

	if gldb, err = leveldb.OpenFile(Config.Goleveldb.DbFile, nil); err != nil {
		log.Fatalln("leveldb.OpenFile failed")
	}
}

func gldbDump() {
	iter := gldb.NewIterator(nil, nil)
	for iter.Next() {
		log.Printf("%x: %x\n", iter.Key(), iter.Value())
	}
	iter.Release()
}

// mysql - must have mysql installed and running and set up as follows.
// (these names are all configurable in config.toml)
//
//	mysql -u root -p
//	create database testSPA;
//	create user 'spaTestUser'@'localhost' identified with mysql_native_password by 'hello';
//	grant event, create, select, insert, update, delete, drop on testSPA.* to 'spaTestUser'@'localhost';
//	grant super on *.* to 'spaTestUser'@'localhost';
func mysqlSetup() {
	var err error

	if sdb != nil {
		return
	}

	s := fmt.Sprintf("%s:%s@/%s?parseTime=true", Config.Mysql.User, Config.Mysql.Password, Config.Mysql.Database)
	if sdb, err = sql.Open("mysql", s); err != nil {
		log.Fatalln("sql.Open failed")
	}

	sdb.SetMaxOpenConns(Config.Mysql.MaxOpenConns)
	sdb.SetMaxIdleConns(Config.Mysql.MaxIdleConns)
}

// postgresql - must have postgresql installed and running and set up as follows.
// (these names are all configurable in config.toml)
//
//	sudo -u postgres psql
//	create user spatestuser with password 'hello';
//	create database testspa;
//	grant all privileges on database testspa to spatestuser;
func postgresqlSetup() {
	var err error

	if pdb != nil {
		return
	}

	url := fmt.Sprintf("postgresql://%s:%s@localhost:5432/%s", Config.Postgresql.User, Config.Postgresql.Password, Config.Postgresql.Database)

	pcfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatalln("pgxpool.ParseConfig failed - " + err.Error())
	}

	pcfg.MaxConns = int32(Config.Postgresql.MaxConns)

	pdb, err = pgxpool.NewWithConfig(noctx, pcfg)
	if err != nil {
		log.Fatalln("pgxpool.NewWithConfig failed - " + err.Error())
	}
}

// cassandra - must have cassandra installed and running.
// By default, cassandra is accessible without a user/password.
// To clean up, launch cqlsh and execute: DROP KEYSPACE excass;
func cassandraSetup() {
	var err error

	if cdb != nil {
		return
	}

	cluster := gocql.NewCluster(Config.Cassandra.ClusterAddr)
	cluster.ProtoVersion = Config.Cassandra.ProtoVersion
	cluster.Consistency = gocql.ParseConsistency(Config.Cassandra.Consistency)
	// each try is this long and you get 10 retries
	cluster.Timeout = time.Duration(Config.Cassandra.TimeoutMsec) * time.Millisecond

	// session for creating the keyspace
	cdb, err = cluster.CreateSession()
	if err != nil {
		log.Fatalln("dbSetup - " + err.Error())
	}
	cdb.Query(`CREATE KEYSPACE IF NOT EXISTS ` + Config.Cassandra.KeySpace + ` WITH REPLICATION = 
		{ 'class' : 'SimpleStrategy', 'replication_factor' : 1 };`).Exec()
	cdb.Close()

	// session for the rest of setup and for on-going operation
	cluster.Keyspace = Config.Cassandra.KeySpace
	cdb, err = cluster.CreateSession()
}
