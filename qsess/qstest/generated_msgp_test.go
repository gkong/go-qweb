package qstest

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import "github.com/tinylib/msgp/msgp"

// MarshalMsg implements msgp.Marshaler
func (z *MsgpData) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Userid"
	o = append(o, 0x83, 0xa6, 0x55, 0x73, 0x65, 0x72, 0x69, 0x64)
	o = msgp.AppendBytes(o, z.Userid[:])
	// string "Username"
	o = append(o, 0xa8, 0x55, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Username)
	// string "Note"
	o = append(o, 0xa4, 0x4e, 0x6f, 0x74, 0x65)
	o = msgp.AppendString(o, z.Note)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *MsgpData) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zbzg uint32
	zbzg, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zbzg > 0 {
		zbzg--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Userid":
			bts, err = msgp.ReadExactBytes(bts, z.Userid[:])
			if err != nil {
				return
			}
		case "Username":
			z.Username, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Note":
			z.Note, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *MsgpData) Msgsize() (s int) {
	s = 1 + 7 + msgp.ArrayHeaderSize + (16 * (msgp.ByteSize)) + 9 + msgp.StringPrefixSize + len(z.Username) + 5 + msgp.StringPrefixSize + len(z.Note)
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *MsgpUUID) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendBytes(o, z[:])
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *MsgpUUID) UnmarshalMsg(bts []byte) (o []byte, err error) {
	bts, err = msgp.ReadExactBytes(bts, z[:])
	if err != nil {
		return
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *MsgpUUID) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize + (16 * (msgp.ByteSize))
	return
}
