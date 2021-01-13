// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// to re-generate generated_msgp.go:
//    install github.com/tinylib/msgp
//    run "go generate"

//go:generate msgp -o=generated_msgp_test.go -io=false -tests=false

package qstest

import (
	"fmt"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

// MsgpData is a clone of mySessData, for use with tinylib/msgp.
// Need to clone, so we can provide different Marshal/Unmarshal methods.
type MsgpData struct {
	// example application-defined session variables
	Userid   MsgpUUID
	Username string
	Note     string
}

func newMsgpData() qsess.SessData {
	return &MsgpData{}
}

func (md *MsgpData) Marshal() ([]byte, error) {
	return md.MarshalMsg([]byte{})
}

func (md *MsgpData) Unmarshal(b []byte) error {
	_, err := md.UnmarshalMsg(b)
	return err
}

var tinylibmsgpHasRun bool

func BenchmarkTinylibMsgp(b *testing.B) {
	userID := MsgpUUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5}
	st, _ := qsess.NewStore(nil, false, []byte("key-to-encrypt------------------"))
	st.NewSessData = newMsgpData
	var sess struct{ Data qsess.SessData }
	var sdata *MsgpData

	for i := 0; i < b.N; i++ {
		sess.Data = st.NewSessData()
		sdata = sess.Data.(*MsgpData)

		sdata.Userid = userID
		sdata.Username = "JohnDoe"
		sdata.Note = "This is an example string of 43 characters."

		roundTrip(b, sdata)
	}

	if !tinylibmsgpHasRun {
		endMessage = endMessage + fmt.Sprintf("TinyLibMsgp serialized size = %d\n", serSize(b, sdata))
		tinylibmsgpHasRun = true
	}
}

type MsgpUUID [16]byte

func (u MsgpUUID) String() string {
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

func (u MsgpUUID) Bytes() []byte {
	return u[:]
}
