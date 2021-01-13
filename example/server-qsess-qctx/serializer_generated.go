package main

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (

	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *MySessData) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zxvk uint32
	zxvk, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zxvk > 0 {
		zxvk--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Userid":
			z.Userid, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "Username":
			z.Username, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Note":
			z.Note, err = dc.ReadString()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z MySessData) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "Userid"
	err = en.Append(0x83, 0xa6, 0x55, 0x73, 0x65, 0x72, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.Userid)
	if err != nil {
		return
	}
	// write "Username"
	err = en.Append(0xa8, 0x55, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Username)
	if err != nil {
		return
	}
	// write "Note"
	err = en.Append(0xa4, 0x4e, 0x6f, 0x74, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Note)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z MySessData) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Userid"
	o = append(o, 0x83, 0xa6, 0x55, 0x73, 0x65, 0x72, 0x69, 0x64)
	o = msgp.AppendInt64(o, z.Userid)
	// string "Username"
	o = append(o, 0xa8, 0x55, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Username)
	// string "Note"
	o = append(o, 0xa4, 0x4e, 0x6f, 0x74, 0x65)
	o = msgp.AppendString(o, z.Note)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *MySessData) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
			z.Userid, bts, err = msgp.ReadInt64Bytes(bts)
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
func (z MySessData) Msgsize() (s int) {
	s = 1 + 7 + msgp.Int64Size + 9 + msgp.StringPrefixSize + len(z.Username) + 5 + msgp.StringPrefixSize + len(z.Note)
	return
}
