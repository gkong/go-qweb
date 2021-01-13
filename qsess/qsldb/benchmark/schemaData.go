// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

//go:generate zebrapack -o=schemaData_generated.go -no-structnames-onwire -tests=false

package bench

type BenchSessVal struct {
	Expiration int64  `zid:"0"`
	MaxAge     int    `zid:"1"`
	MinRefresh int    `zid:"2"`
	Data       []byte `zid:"3"`
}
