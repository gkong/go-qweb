// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Session data benchmarks, using a small example struct:
//	qsess default serializer - gob
//  tinylib/msgp - messagepack
//  zebrapack
//  hand-written

package qstest

import (
	"bytes"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/gkong/go-qweb/qsess"
)

var endMessage = "\n"

func TestMain(m *testing.M) {
	flag.Parse()
	ret := m.Run()
	fmt.Fprintln(os.Stderr, endMessage)
	os.Exit(ret)
}

// mySessData is an example custom session data type implementation.
type mySessData struct {
	// example application-defined session variables
	Userid   MyUUID
	Username string
	Note     string
}

func newMySessData() qsess.SessData {
	return &mySessData{}
}

// hand-written serializer/deserializer - one-byte length for each item, followed by item data

func (m *mySessData) Marshal() ([]byte, error) {
	u := m.Userid.Bytes()
	idLen := len(u)
	nameLen := len(m.Username)
	noteLen := len(m.Note)
	totalLen := 3 + idLen + nameLen + noteLen
	if idLen > 255 || nameLen > 255 || noteLen > 255 {
		return []byte{}, errors.New("mySessData.Marshal - element too big")
	}
	b := make([]byte, totalLen)
	b[0] = byte(idLen)
	b[1] = byte(nameLen)
	b[2] = byte(noteLen)
	copy(b[3:3+idLen], u)
	copy(b[3+idLen:3+idLen+nameLen], m.Username)
	copy(b[3+idLen+nameLen:totalLen], m.Note)
	return b, nil
}

func (m *mySessData) Unmarshal(b []byte) error {
	if len(b) < 3 {
		return errors.New("mySessData.Unmarshal data malformed")
	}
	idLen := int(b[0])
	nameLen := int(b[1])
	noteLen := int(b[2])
	totalLen := 3 + idLen + nameLen + noteLen
	if len(b) != totalLen {
		return errors.New("mySessData.Unmarshal data wrong size")
	}
	copy(m.Userid[:], b[3:3+idLen])
	m.Username = string(b[3+idLen : 3+idLen+nameLen])
	m.Note = string(b[3+idLen+nameLen : totalLen])
	return nil
}

var handwrittenHasRun bool

func BenchmarkHandWrittenSerializer(b *testing.B) {
	userID := MyUUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5}
	st, _ := qsess.NewStore(nil, false, []byte("key-to-encrypt------------------"))
	st.NewSessData = newMySessData
	var sess struct{ Data qsess.SessData }
	var sdata *mySessData

	for i := 0; i < b.N; i++ {
		sess.Data = st.NewSessData()
		sdata = sess.Data.(*mySessData)

		sdata.Userid = userID
		sdata.Username = "JohnDoe"
		sdata.Note = "This is an example string of 43 characters."

		roundTrip(b, sdata)
	}

	if !handwrittenHasRun {
		endMessage = endMessage + fmt.Sprintf("HandWritten serialized size = %d\n", serSize(b, sdata))
		handwrittenHasRun = true
	}
}

func newJSONData() qsess.SessData {
	return &JSONData{}
}

var JSONHasRun bool

func BenchmarkJSON(b *testing.B) {
	userID := JSONUUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5}
	st, _ := qsess.NewStore(nil, false, []byte("key-to-encrypt------------------"))
	st.NewSessData = newJSONData
	var sess struct{ Data qsess.SessData }
	var sdata *JSONData

	for i := 0; i < b.N; i++ {
		sess.Data = st.NewSessData()
		sdata = sess.Data.(*JSONData)

		sdata.Userid = userID
		sdata.Username = "JohnDoe"
		sdata.Note = "This is an example string of 43 characters."

		roundTrip(b, sdata)
	}

	if !JSONHasRun {
		endMessage = endMessage + fmt.Sprintf("JSON serialized size = %d\n", serSize(b, sdata))
		JSONHasRun = true
	}
}

func newZebraData() qsess.SessData {
	return &ZebraData{}
}

func (z *ZebraData) Marshal() ([]byte, error) {
	return z.MarshalMsg([]byte{})
}

func (z *ZebraData) Unmarshal(b []byte) error {
	_, err := z.UnmarshalMsg(b)
	return err
}

var zebraHasRun bool

func BenchmarkZebra(b *testing.B) {
	userID := ZebraUUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5}
	st, _ := qsess.NewStore(nil, false, []byte("key-to-encrypt------------------"))
	st.NewSessData = newZebraData
	var sess struct{ Data qsess.SessData }
	var sdata *ZebraData

	for i := 0; i < b.N; i++ {
		sess.Data = st.NewSessData()
		sdata = sess.Data.(*ZebraData)

		sdata.Userid = userID
		sdata.Username = "JohnDoe"
		sdata.Note = "This is an example string of 43 characters."

		roundTrip(b, sdata)
	}

	if !zebraHasRun {
		endMessage = endMessage + fmt.Sprintf("Zebra serialized size = %d\n", serSize(b, sdata))
		zebraHasRun = true
	}
}

// Unsafe version of VarMap, which re-uses gob encoders and decoders,
// by maintaining a sync.Pool of encoder/decoder pairs.
//
// We allocate them in pairs, so we can run one round-trip through
// them, to get them in sync, before putting them into service.
//
// This depends on each pair agreeing on the same identifying number for
// our data structure. Empirically, this works, but I don't know of any
// promises that it will always work, or work the same from release to release.

type VarMap2 struct {
	Vars map[interface{}]interface{}
}

func newVarMap2() qsess.SessData {
	return &VarMap2{make(map[interface{}]interface{})}
}

func (m *VarMap2) Marshal() ([]byte, error) {
	vme := gobPool.Get().(*VarMapCodec)
	defer func() {
		vme.buf.Reset()
		gobPool.Put(vme)
	}()

	if err := vme.enc.Encode(&m.Vars); err != nil {
		return []byte{}, fmt.Errorf("VarMap.Marshal - Encode failed - %s", err.Error())
	}

	b := make([]byte, vme.buf.Len())
	copy(b, vme.buf.Bytes())
	return b, nil
}

func (m *VarMap2) Unmarshal(b []byte) error {
	vme := gobPool.Get().(*VarMapCodec)
	defer func() {
		vme.buf.Reset()
		gobPool.Put(vme)
	}()

	if _, err := vme.buf.Write(b); err != nil {
		return fmt.Errorf("VarMap.Unmarshal - buffer Write failed - %s", err.Error())
	}

	return vme.dec.Decode(&m.Vars)
}

type VarMapCodec struct {
	buf bytes.Buffer
	enc *gob.Encoder
	dec *gob.Decoder
}

func newVarMapCodec() interface{} {
	vmc := VarMapCodec{buf: bytes.Buffer{}}
	vmc.enc = gob.NewEncoder(&vmc.buf)
	vmc.dec = gob.NewDecoder(&vmc.buf)

	// prime the pump
	m := make(map[interface{}]interface{})
	if err := vmc.enc.Encode(&m); err != nil {
		panic("newVarMapCodec - Encode failed - " + err.Error())
	}
	if err := vmc.dec.Decode(&m); err != nil {
		panic("newVarMapCodec - Decode failed - " + err.Error())
	}
	vmc.buf.Reset()
	return &vmc
}

var gobPool = sync.Pool{New: newVarMapCodec} // this would live in Store if using for real

var unsafegobHasRun bool

func BenchmarkUnsafeGobWithReuse(b *testing.B) {
	userID := MyUUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5}
	st, _ := qsess.NewStore(nil, false, []byte("key-to-encrypt------------------"))
	st.NewSessData = newVarMap2
	var sess struct{ Data qsess.SessData }
	var sdata *VarMap2

	for i := 0; i < b.N; i++ {
		sess.Data = st.NewSessData()
		sdata = sess.Data.(*VarMap2)

		sdata.Vars["userid"] = userID.String()
		sdata.Vars["username"] = "JohnDoe"
		sdata.Vars["note"] = "This is an example string of 43 characters."

		roundTrip(b, sdata)
	}

	if !unsafegobHasRun {
		endMessage = endMessage + fmt.Sprintf("UnsafeGobWithReuse serialized size = %d\n", serSize(b, sdata))
		unsafegobHasRun = true
	}
}

var safegobHasRun bool

func BenchmarkDefaultSafeGob(b *testing.B) {
	userID := MyUUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5}
	st, _ := qsess.NewStore(nil, false, []byte("key-to-encrypt------------------"))
	var sess struct{ Data qsess.SessData }
	var sdata *qsess.VarMap

	for i := 0; i < b.N; i++ {
		sess.Data = st.NewSessData()
		sdata = sess.Data.(*qsess.VarMap)

		sdata.Vars["userid"] = userID.String()
		sdata.Vars["username"] = "JohnDoe"
		sdata.Vars["note"] = "This is an example string of 43 characters."

		roundTrip(b, sdata)
	}

	if !safegobHasRun {
		endMessage = endMessage + fmt.Sprintf("DefaultSafeGob serialized size = %d\n", serSize(b, sdata))
		safegobHasRun = true
	}
}

// serialize and immediately deserialize
func roundTrip(b *testing.B, sdata qsess.SessData) {
	ser, mErr := sdata.Marshal()
	if mErr != nil {
		b.Fatal(mErr)
	}

	if err := sdata.Unmarshal(ser); err != nil {
		b.Fatal(err)
	}
}

// return the size of serialized data
func serSize(b *testing.B, sdata qsess.SessData) int {
	ser, err := sdata.Marshal()
	if err != nil {
		b.Fatal(err)
	}
	return len(ser)
}

// uuid code borrowed from gocql

type MyUUID [16]byte

func (u MyUUID) String() string {
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

func (u MyUUID) Bytes() []byte {
	return u[:]
}
