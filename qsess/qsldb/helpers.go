// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsldb

import (
	"crypto/rand"
	"encoding/binary"
	"io"
)

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

// convert int64 to []byte
func itob(dest []byte, i int64) {
	binary.LittleEndian.PutUint64(dest, uint64(i))
}

// convert []byte to int64
func btoi(b []byte) int64 {
	return int64(binary.LittleEndian.Uint64(b))
}

func randomBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("qsldb.randomBytes - cannot read rand.Reader")
	}
	return b
}

type gldbErr struct {
	msg string
	err error
}

func (e gldbErr) Error() string {
	if e.err != nil {
		return "qsldb." + e.msg + " - " + e.err.Error()
	}
	return "qsldb." + e.msg
}
