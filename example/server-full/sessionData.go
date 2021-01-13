// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

// Data type for session data with hand-written serializer.

package main

import (
	"errors"

	"github.com/gkong/go-qweb/qsess"
)

type mySessData struct {
	userid   []byte
	username string
	note     string
}

func newMySessData() qsess.SessData {
	return &mySessData{}
}

// simple struct-specific serializer/deserializer - one-byte length for each item, followed by item data
func (m *mySessData) Marshal() ([]byte, error) {
	idLen := len(m.userid)
	nameLen := len(m.username)
	noteLen := len(m.note)
	totalLen := 3 + idLen + nameLen + noteLen
	if idLen > 255 || nameLen > 255 || noteLen > 255 {
		return []byte{}, errors.New("mySessData.Marshal - element too big")
	}
	b := make([]byte, totalLen)
	b[0] = byte(idLen)
	b[1] = byte(nameLen)
	b[2] = byte(noteLen)
	copy(b[3:3+idLen], m.userid)
	copy(b[3+idLen:3+idLen+nameLen], m.username)
	copy(b[3+idLen+nameLen:totalLen], m.note)
	return b, nil
}

func (m *mySessData) Unmarshal(b []byte) error {
	if len(b) < 3 {
		return errors.New("mySessData.Unmarshal - data malformed")
	}
	idLen := int(b[0])
	nameLen := int(b[1])
	noteLen := int(b[2])
	totalLen := 3 + idLen + nameLen + noteLen
	if len(b) != totalLen {
		return errors.New("mySessData.Unmarshal data wrong size")
	}
	copy(m.userid, b[3:3+idLen])
	m.username = string(b[3+idLen : 3+idLen+nameLen])
	m.note = string(b[3+idLen+nameLen : totalLen])
	return nil
}
