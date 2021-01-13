// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// compare methods for serializing structured data into/out of goleveldb

package bench

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"testing"
)

var endMessage string = "\n"

func TestMain(m *testing.M) {
	flag.Parse()
	ret := m.Run()
	fmt.Fprintln(os.Stderr, endMessage)
	os.Exit(ret)
}

var benchData = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
}

const bytesPerInt64 = 8

// Session Table
//
//   key: prefix | opaque key
//
//   value: expiration time | maxage | minrefresh | variable-sized data
//     (the first 3 fields are all int64s)

type gldbSessValue []byte

const sessValueFixedPartSize = 3 * bytesPerInt64

func newSessValue(datasize int) gldbSessValue {
	return gldbSessValue(make([]byte, sessValueFixedPartSize+datasize))
}

func (v *gldbSessValue) expiration() int64 {
	return btoi((*v)[:bytesPerInt64])
}

func (v *gldbSessValue) expirationBytes() []byte {
	return (*v)[:bytesPerInt64]
}

func (v *gldbSessValue) maxage() int64 {
	return btoi((*v)[bytesPerInt64 : 2*bytesPerInt64])
}

func (v *gldbSessValue) maxageBytes() []byte {
	return (*v)[bytesPerInt64 : 2*bytesPerInt64]
}

func (v *gldbSessValue) minrefresh() int64 {
	return btoi((*v)[2*bytesPerInt64 : 3*bytesPerInt64])
}

func (v *gldbSessValue) minrefreshBytes() []byte {
	return (*v)[2*bytesPerInt64 : 3*bytesPerInt64]
}

func (v *gldbSessValue) data() []byte {
	return (*v)[3*bytesPerInt64:]
}

var bsaHasRun bool

func BenchmarkByteSliceAccessors(b *testing.B) {
	var maxAgeSecs int = 5
	var minRefreshSecs int = 2
	var expire int64 = 42

	if !bsaHasRun {
		bsaHasRun = true
		endMessage = endMessage + fmt.Sprintf("ByteSliceAccessors size %d\n", len(newSessValue(len(benchData))))
	}

	for i := 0; i < b.N; i++ {
		// marshal
		wVal := newSessValue(len(benchData))
		itob(wVal.expirationBytes(), expire)
		itob(wVal.maxageBytes(), int64(maxAgeSecs))
		itob(wVal.minrefreshBytes(), int64(minRefreshSecs))
		copy(wVal.data(), benchData)

		// unmarshal
		if len(wVal) < sessValueFixedPartSize {
			// test will always pass here, but is necessary in the wild
			b.Fatal("malformed session record")
		}
		rVal := gldbSessValue(wVal)
		doNothing(rVal.data(), rVal.expiration(), int(rVal.maxage()), int(rVal.minrefresh()))
	}
}

var bscatHasRun bool

func BenchmarkBscat(b *testing.B) {
	var maxAgeSecs int = 5
	var minRefreshSecs int = 2
	var expire int64 = 42

	if !bscatHasRun {
		bscatHasRun = true
		endMessage = endMessage + fmt.Sprintf("Bscat size %d\n", len(bscat(ritob(expire), ritob(int64(maxAgeSecs)), ritob(int64(minRefreshSecs)), benchData)))
	}

	for i := 0; i < b.N; i++ {
		// marshal
		wVal := bscat(ritob(expire), ritob(int64(maxAgeSecs)), ritob(int64(minRefreshSecs)), benchData)

		// unmarshal
		if len(wVal) < sessValueFixedPartSize {
			b.Fatal("malformed session record")
		}
		rVal := gldbSessValue(wVal)
		doNothing(rVal.data(), rVal.expiration(), int(rVal.maxage()), int(rVal.minrefresh()))
	}
}

var bytesjoinHasRun bool

func BenchmarkBytesJoin(b *testing.B) {
	var maxAgeSecs int = 5
	var minRefreshSecs int = 2
	var expire int64 = 42

	if !bytesjoinHasRun {
		bytesjoinHasRun = true
		bss := [][]byte{ritob(expire), ritob(int64(maxAgeSecs)), ritob(int64(minRefreshSecs)), benchData}
		endMessage = endMessage + fmt.Sprintf("BytesJoin size %d\n", len(bytes.Join(bss, []byte{})))
	}

	for i := 0; i < b.N; i++ {
		// marshal
		bss := [][]byte{ritob(expire), ritob(int64(maxAgeSecs)), ritob(int64(minRefreshSecs)), benchData}
		wVal := bytes.Join(bss, []byte{})

		// unmarshal
		if len(wVal) < sessValueFixedPartSize {
			b.Fatal("malformed session record")
		}
		rVal := gldbSessValue(wVal)
		doNothing(rVal.data(), rVal.expiration(), int(rVal.maxage()), int(rVal.minrefresh()))
	}
}

var zebraHasRun bool

func BenchmarkZebraPack(b *testing.B) {
	var maxAgeSecs int = 5
	var minRefreshSecs int = 2
	var expire int64 = 42

	if !zebraHasRun {
		zebraHasRun = true
		sv1 := BenchSessVal{expire, maxAgeSecs, minRefreshSecs, benchData[:]}
		marshalled1, _ := sv1.MarshalMsg([]byte{})
		endMessage = endMessage + fmt.Sprintf("ZebraPack size %d\n", len(marshalled1))
	}

	for i := 0; i < b.N; i++ {

		sv := BenchSessVal{expire, maxAgeSecs, minRefreshSecs, benchData[:]}
		marshalled, err := sv.MarshalMsg([]byte{})
		if err != nil {
			b.Fatal("MarshalMsg error")
		}

		rv := BenchSessVal{}
		_, err = rv.UnmarshalMsg(marshalled)
		if err != nil {
			b.Fatal("UnMarshalMsg error")
		}

		doNothing(rv.Data, rv.Expiration, rv.MaxAge, rv.MinRefresh)
	}
}

func doNothing(b []byte, exp int64, maxage int, minreferesh int) int {
	return maxage
}

// convert int64 to []byte and copy to given byte slice
func itob(dest []byte, i int64) {
	binary.LittleEndian.PutUint64(dest, uint64(i))
}

// convert int64 to []byte and return it in a new byte slice
func ritob(i int64) []byte {
	b := make([]byte, bytesPerInt64)
	binary.LittleEndian.PutUint64(b, uint64(i))
	return b
}

// convert []byte to int64
func btoi(b []byte) int64 {
	return int64(binary.LittleEndian.Uint64(b))
}

// concatenate byte slices, returning a new one containing the contents of all
func bscat(bb ...[]byte) []byte {
	size := 0
	for _, b := range bb {
		size += len(b)
	}

	ret := make([]byte, size)
	pos := 0
	for _, b := range bb {
		copy(ret[pos:], b)
		pos += len(b)
	}

	return ret
}
