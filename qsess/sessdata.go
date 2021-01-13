// Copyright 2016 George S. Kong. All rights reserved.
// Use of this source code is governed by a license that can be found in the LICENSE.txt file.

package qsess

import (
	"bytes"
	"encoding/gob"
)

// VarMap is the default session data type.
type VarMap struct {
	Vars map[interface{}]interface{}
}

func newVarMap() SessData {
	return &VarMap{make(map[interface{}]interface{})}
}

func (m *VarMap) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(m.Vars)
	return buf.Bytes(), err
}

func (m *VarMap) Unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	return dec.Decode(&m.Vars)
}

// noSessData is a degenerate implementation of SessData, which stores no data.
// it is what you get when you set NewSessData to nil.
// it's useful when the only data you need to maintain is a user id,
// which you can set via NewSession and retrieve via UserID

type noSessData struct{}

func (n noSessData) Marshal() ([]byte, error) {
	return []byte{}, nil
}

func (n noSessData) Unmarshal(b []byte) error {
	return nil
}
