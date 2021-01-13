// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// to regenerate generated_zebra.go:
//   install github.com/glycerine/zebrapack
//    run "go generate"

//go:generate zebrapack -o=generated_zebra_test.go -io=false -no-structnames-onwire -tests=false

package qstest

type ZebraData struct {
	// example application-defined session variables
	Userid   ZebraUUID `zid:"0"`
	Username string    `zid:"1"`
	Note     string    `zid:"2"`
}

type ZebraUUID [16]byte

func (u ZebraUUID) String() string {
	var offsets = [...]int{0, 2, 4, 6, 9, 11, 14, 16, 19, 21, 24, 26, 28, 30, 32, 34}
	const hexString = "0123456789abcdef"
	r := make([]byte, 36)
	for i, b := range u {
		r[offsets[i]] = hexString[b>>4]
		r[offsets[i]+1] = hexString[b&0xF]
	}
	r[8] = '-'
	r[13] = '-'
	r[18] = '-'
	r[23] = '-'
	return string(r)

}

func (u ZebraUUID) Bytes() []byte {
	return u[:]
}
