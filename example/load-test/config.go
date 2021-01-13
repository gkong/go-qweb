// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

// XXX - some day make a library to reduce repetition. would use multiconfig, if it doesn't handle maps.

const configFile = "./config.toml"

// toml file schema

var Config tomlConfig

type tomlConfig struct {
	Scenario string `required:"true"` // name of the chosen scenario

	RootURL string `required:"true"`

	MaxPorts  int `required:"true"` // number of ports available in the system
	MaxActive int `required:"true"` // max active users, based on ports, open files, etc.

	ServerExit bool // when we shut down, send a shutdown request to the server.

	CpuProfFile string // if non-empty, it's name of file in which to save cpu profiling data
	MemProfFile string // if non-empty, it's name of file in which to save mem profiling data

	Scenarios map[string]Scenario
}

type Scenario struct {
	TestSecs  int `required:"true"` // total time to run the test
	NumUsers  int `required:"true"`
	MsBetwReq int `required:"true"` // mean time between http requests for a given user
	ShowStats bool
	ShowWait  bool // show avg wait time (meaningless with heavy load)
	AlwaysOn  bool // do NOT alternate between activity and rest.
	// these constants are only used if alwaysOn is false
	ActivitiesPerDay int
	SecsPerActivity  int
	FracLogout       float64 // fraction of activities which end with a log-out
}

var scenario Scenario

// command-line flags

var fHelp bool

var fScenario string
var fRootURL string
var fMaxPorts int
var fMaxActive int
var fServerExit bool
var fCPUProfFile string
var fMemProfFile string

var fTestSecs int
var fNumUsers int
var fMsBetwReq int
var fShowStats bool
var fShowWait bool
var fAlwaysOn bool
var fActivitiesPerDay int
var fSecsPerActivity int
var fFracLogout float64

func doFlags() {
	var ok bool

	flag.BoolVar(&fHelp, "help", false, "print this help message")

	flag.StringVar(&fScenario, "scenario", "", "name of scenario to be used")

	flag.StringVar(&fRootURL, "rooturl", "", "")
	flag.IntVar(&fMaxPorts, "maxports", -1, "number of ports available in the system")
	flag.IntVar(&fMaxActive, "maxactive", -1, "max active users, based on ports, open files, etc.")
	flag.BoolVar(&fServerExit, "serverexit", false, "when we shut down, send a shutdown request to the server")
	flag.StringVar(&fCPUProfFile, "cpuproffile", "", "name of file in which to save cpu profiling data")
	flag.StringVar(&fMemProfFile, "memproffile", "", "name of file in which to save mem profiling data")

	flag.IntVar(&fTestSecs, "testsecs", -1, "length of time to run the simulation")
	flag.IntVar(&fNumUsers, "numusers", -1, "number of simulated users")
	flag.IntVar(&fMsBetwReq, "msbetwreq", -1, "mean time between http requests for a given user")

	flag.BoolVar(&fShowStats, "showstats", false, "print statistics every second")
	flag.BoolVar(&fShowWait, "showwait", false, "show avg wait time (meaningless with heavy load)")
	flag.BoolVar(&fAlwaysOn, "alwayson", false, "do NOT alternate between activity and rest")

	flag.IntVar(&fActivitiesPerDay, "activitiesperday", -1, "average number of active periods per user per day")
	flag.IntVar(&fSecsPerActivity, "secsperactivity", -1, "average duration of an active period")
	flag.Float64Var(&fFracLogout, "fraclogout", -1.0, "fraction of activities which end with a log-out")

	flag.Parse()

	if fHelp {
		help()
		os.Exit(0)
	}

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "scenario" {
			Config.Scenario = fScenario
		}
	})

	scenario, ok = Config.Scenarios[Config.Scenario]
	if !ok {
		log.Fatalln("selected scenario does not exist")
	}

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "rooturl":
			Config.RootURL = fRootURL
		case "maxports":
			Config.MaxPorts = fMaxPorts
		case "maxactive":
			Config.MaxActive = fMaxActive
		case "serverexit":
			Config.ServerExit = fServerExit
		case "cpuproffile":
			Config.CpuProfFile = fCPUProfFile
		case "memproffile":
			Config.MemProfFile = fMemProfFile

		case "testsecs":
			scenario.TestSecs = fTestSecs
		case "numusers":
			scenario.NumUsers = fNumUsers
		case "msbetwreq":
			scenario.MsBetwReq = fMsBetwReq
		case "showstats":
			scenario.ShowStats = fShowStats
		case "showwait":
			scenario.ShowWait = fShowWait
		case "alwayson":
			scenario.AlwaysOn = fAlwaysOn
		case "activitiesperday":
			scenario.ActivitiesPerDay = fActivitiesPerDay
		case "secsperactivity":
			scenario.SecsPerActivity = fSecsPerActivity
		case "fraclogout":
			scenario.FracLogout = fFracLogout
		}
	})
}

func help() {
	fmt.Print("\ncommand-line flags override settings found in: ", configFile, "\n\n")

	fmt.Print("scenarios available: ")
	keys := make([]string, 0, len(Config.Scenarios))
	for key := range Config.Scenarios {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	first := true
	for _, key := range keys {
		if !first {
			fmt.Print(", ")
		}
		fmt.Print(key)
		first = false
	}
	fmt.Print("\n\n")

	printUsages()
}

func doConfig() {
	var ok bool

	if _, err := toml.DecodeFile(configFile, &Config); err != nil {
		log.Fatalln("toml.Decode failed - ", err.Error())
	}

	scenario, ok = Config.Scenarios[Config.Scenario]
	if !ok {
		log.Fatalln("selected scenario does not exist")
	}

	doFlags()

	users = make([]user, scenario.NumUsers)

	maxIdle = Config.MaxPorts / 2

	if !scenario.AlwaysOn {
		avgWaitSecs = ((24 * 60 * 60) -
			(scenario.ActivitiesPerDay * scenario.SecsPerActivity)) / scenario.ActivitiesPerDay
	}

	log.Printf("starting scenario %s\n", Config.Scenario)
}

// cloned from flag.PrintDefaults, removed printing of default value
func printUsages() {
	flag.VisitAll(func(f *flag.Flag) {
		s := fmt.Sprintf("  -%s", f.Name)
		name, usage := flag.UnquoteUsage(f)
		if len(name) > 0 {
			s += " " + name
		}
		if len(s) <= 4 { // space, space, '-', 'x'.
			s += "\t"
		} else {
			s += "\n    \t"
		}
		s += usage
		fmt.Println(s)
	})
}
