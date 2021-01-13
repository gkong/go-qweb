// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Database Schema
//
// Probably should use an abstraction layer here, but, in the interest
// of minimizing dependencies, we just use hand-written accessor functions
// to manipulate fields within byte-slice buffers. This also has the advantage
// of good performance.

package qsldb

import (
	"time"
)

const (
	bytesPerInt64   = 8
	sessKeyRandSize = 10
)

// Session Table
//
//   key: prefix | opaque unique key based on time and random data
//
//   value: expiration time | maxage | minrefresh | userid size | variable-sized user id | variable-sized user session data
//     (the first 3 fields are all int64s)

type gldbSessKey []byte

func sessKeySize(prefixSize int) int {
	return prefixSize + sessKeyRandSize + bytesPerInt64
}

func (gst *gldbStore) newSessKey() gldbSessKey {
	key := make([]byte, gst.sessKeySize)
	copy(key[:gst.prefixSize], gst.sessPrefix)
	// Session key = creation time + some random data
	copy(key[gst.prefixSize:], randomBytes(sessKeyRandSize))
	itob(key[gst.prefixSize+sessKeyRandSize:], time.Now().UnixNano())
	return key
}

type gldbSessValue []byte

const sessValueFixedPartSize = 3*bytesPerInt64 + 1

func newSessValue(uidsize int, datasize int) (gldbSessValue, error) {
	if uidsize > 255 {
		return nil, gldbErr{"newSessValue - uidsize must be <= 255", nil}
	}
	v := gldbSessValue(make([]byte, sessValueFixedPartSize+uidsize+datasize))
	v.uidsizeBytes()[0] = byte(uidsize)
	return v, nil
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

func (v *gldbSessValue) uidsize() int {
	return int((*v)[3*bytesPerInt64])
}

func (v *gldbSessValue) uidsizeBytes() []byte {
	return (*v)[3*bytesPerInt64 : (3*bytesPerInt64)+1]
}

func (v *gldbSessValue) userID() []byte {
	return (*v)[(3*bytesPerInt64)+1 : (3*bytesPerInt64)+1+v.uidsize()]
}

func (v *gldbSessValue) data() []byte {
	return (*v)[(3*bytesPerInt64)+1+v.uidsize():]
}

// index by expiration time
//
//   key: prefix | expiration time | session key
//
//   value: (empty)

type gldbExpKey []byte

func (gst *gldbStore) expKey(expires []byte, sessKey []byte) gldbExpKey {
	return bscat(gst.expPrefix, expires, sessKey)
}

func expKeySize(prefixSize int) int {
	return prefixSize + bytesPerInt64 + sessKeySize(prefixSize)
}

func (k *gldbExpKey) expiration(prefixSize int) int64 {
	return btoi((*k)[prefixSize : prefixSize+bytesPerInt64])
}

func (k *gldbExpKey) sessKey(prefixSize int) []byte {
	return (*k)[prefixSize+bytesPerInt64:]
}

// index by user id
//
//   key: prefix | user id | session key
//
//   value: (empty)

type gldbUIDKey []byte

func (gst *gldbStore) uidKey(userID []byte, sessKey []byte) gldbUIDKey {
	return bscat(gst.uidPrefix, userID, sessKey)
}

// prefix for searching by userID
func (gst *gldbStore) uidKeyPrefix(userID []byte) gldbUIDKey {
	return bscat(gst.uidPrefix, userID)
}

func (k *gldbUIDKey) sessKey(prefixSize int) []byte {
	return (*k)[len(*k)-sessKeySize(prefixSize):]
}
