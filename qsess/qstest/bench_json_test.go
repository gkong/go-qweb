// Copyright 2021 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qstest

import "encoding/json"

type JSONData struct {
	// example application-defined session variables
	Userid   JSONUUID `json:"uuid"`
	Username string   `json:"username"`
	Note     string   `json:"note"`
}

func (j *JSONData) Marshal() ([]byte, error) {
	return json.Marshal(j)
}

func (j *JSONData) Unmarshal(b []byte) error {
	return json.Unmarshal(b, j)
}

type JSONUUID [16]byte

func (u JSONUUID) String() string {
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

func (u JSONUUID) Bytes() []byte {
	return u[:]
}
