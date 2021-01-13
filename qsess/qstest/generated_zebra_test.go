package qstest

// NOTE: THIS FILE WAS PRODUCED BY THE
// ZEBRAPACK CODE GENERATION TOOL (github.com/glycerine/zebrapack)
// DO NOT EDIT

import "github.com/glycerine/zebrapack/msgp"

// fieldsNotEmpty supports omitempty tags
func (z *ZebraData) fieldsNotEmpty(isempty []bool) uint32 {
	if len(isempty) == 0 {
		return 3
	}
	var fieldsInUse uint32 = 3
	isempty[0] = (len(z.Userid) == 0) // string, omitempty
	if isempty[0] {
		fieldsInUse--
	}
	isempty[1] = (len(z.Username) == 0) // string, omitempty
	if isempty[1] {
		fieldsInUse--
	}
	isempty[2] = (len(z.Note) == 0) // string, omitempty
	if isempty[2] {
		fieldsInUse--
	}

	return fieldsInUse
}

// MarshalMsg implements msgp.Marshaler
func (z *ZebraData) MarshalMsg(b []byte) (o []byte, err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	o = msgp.Require(b, z.Msgsize())

	// honor the omitempty tags
	var empty [3]bool
	fieldsInUse := z.fieldsNotEmpty(empty[:])
	o = msgp.AppendMapHeader(o, fieldsInUse)

	if !empty[0] {
		// zid 0 for "Userid"
		o = append(o, 0x0)
		o = msgp.AppendBytes(o, z.Userid[:])
	}

	if !empty[1] {
		// zid 1 for "Username"
		o = append(o, 0x1)
		o = msgp.AppendString(o, z.Username)
	}

	if !empty[2] {
		// zid 2 for "Note"
		o = append(o, 0x2)
		o = msgp.AppendString(o, z.Note)
	}

	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ZebraData) UnmarshalMsg(bts []byte) (o []byte, err error) {
	return z.UnmarshalMsgWithCfg(bts, nil)
}
func (z *ZebraData) UnmarshalMsgWithCfg(bts []byte, cfg *msgp.RuntimeConfig) (o []byte, err error) {
	var nbs msgp.NilBitsStack
	nbs.Init(cfg)
	var sawTopNil bool
	if msgp.IsNil(bts) {
		sawTopNil = true
		bts = nbs.PushAlwaysNil(bts[1:])
	}

	var field []byte
	_ = field
	const maxFields0zkmq = 3

	// -- templateUnmarshalMsgZid starts here--
	var totalEncodedFields0zkmq uint32
	if !nbs.AlwaysNil {
		totalEncodedFields0zkmq, bts, err = nbs.ReadMapHeaderBytes(bts)
		if err != nil {
			return
		}
	}
	encodedFieldsLeft0zkmq := totalEncodedFields0zkmq
	missingFieldsLeft0zkmq := maxFields0zkmq - totalEncodedFields0zkmq

	var nextMiss0zkmq int = -1
	var found0zkmq [maxFields0zkmq]bool
	var curField0zkmq int

doneWithStruct0zkmq:
	// First fill all the encoded fields, then
	// treat the remaining, missing fields, as Nil.
	for encodedFieldsLeft0zkmq > 0 || missingFieldsLeft0zkmq > 0 {
		//fmt.Printf("encodedFieldsLeft: %v, missingFieldsLeft: %v, found: '%v', fields: '%#v'\n", encodedFieldsLeft0zkmq, missingFieldsLeft0zkmq, msgp.ShowFound(found0zkmq[:]), unmarshalMsgFieldOrder0zkmq)
		if encodedFieldsLeft0zkmq > 0 {
			encodedFieldsLeft0zkmq--
			curField0zkmq, bts, err = nbs.ReadIntBytes(bts)
			if err != nil {
				return
			}
		} else {
			//missing fields need handling
			if nextMiss0zkmq < 0 {
				// set bts to contain just mnil (0xc0)
				bts = nbs.PushAlwaysNil(bts)
				nextMiss0zkmq = 0
			}
			for nextMiss0zkmq < maxFields0zkmq && (found0zkmq[nextMiss0zkmq] || unmarshalMsgFieldSkip0zkmq[nextMiss0zkmq]) {
				nextMiss0zkmq++
			}
			if nextMiss0zkmq == maxFields0zkmq {
				// filled all the empty fields!
				break doneWithStruct0zkmq
			}
			missingFieldsLeft0zkmq--
			curField0zkmq = nextMiss0zkmq
		}
		//fmt.Printf("switching on curField: '%v'\n", curField0zkmq)
		switch curField0zkmq {
		// -- templateUnmarshalMsgZid ends here --

		case 0:
			// zid 0 for "Userid"
			found0zkmq[0] = true
			bts, err = nbs.ReadExactBytes(bts, z.Userid[:])
			if err != nil {
				return
			}
		case 1:
			// zid 1 for "Username"
			found0zkmq[1] = true
			z.Username, bts, err = nbs.ReadStringBytes(bts)

			if err != nil {
				return
			}
		case 2:
			// zid 2 for "Note"
			found0zkmq[2] = true
			z.Note, bts, err = nbs.ReadStringBytes(bts)

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
	if nextMiss0zkmq != -1 {
		bts = nbs.PopAlwaysNil()
	}

	if sawTopNil {
		bts = nbs.PopAlwaysNil()
	}
	o = bts
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// fields of ZebraData
var unmarshalMsgFieldOrder0zkmq = []string{"Userid", "Username", "Note"}

var unmarshalMsgFieldSkip0zkmq = []bool{false, false, false}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ZebraData) Msgsize() (s int) {
	s = 1 + 7 + msgp.ArrayHeaderSize + (16 * (msgp.ByteSize)) + 9 + msgp.StringPrefixSize + len(z.Username) + 5 + msgp.StringPrefixSize + len(z.Note)
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ZebraUUID) MarshalMsg(b []byte) (o []byte, err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendBytes(o, z[:])
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ZebraUUID) UnmarshalMsg(bts []byte) (o []byte, err error) {
	return z.UnmarshalMsgWithCfg(bts, nil)
}
func (z *ZebraUUID) UnmarshalMsgWithCfg(bts []byte, cfg *msgp.RuntimeConfig) (o []byte, err error) {
	var nbs msgp.NilBitsStack
	nbs.Init(cfg)
	var sawTopNil bool
	if msgp.IsNil(bts) {
		sawTopNil = true
		bts = nbs.PushAlwaysNil(bts[1:])
	}

	bts, err = nbs.ReadExactBytes(bts, z[:])
	if err != nil {
		return
	}
	if sawTopNil {
		bts = nbs.PopAlwaysNil()
	}
	o = bts
	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ZebraUUID) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize + (16 * (msgp.ByteSize))
	return
}

// FileGenerated_zebra_test_go holds ZebraPack schema from file 'bench_zebra_test.go'
type FileGenerated_zebra_test_go struct{}

// ZebraSchemaInMsgpack2Format provides the ZebraPack Schema in msgpack2 format, length 539 bytes
func (FileGenerated_zebra_test_go) ZebraSchemaInMsgpack2Format() []byte {
	return []byte{
		0x85, 0xaa, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x50, 0x61,
		0x74, 0x68, 0xb3, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x5f, 0x7a,
		0x65, 0x62, 0x72, 0x61, 0x5f, 0x74, 0x65, 0x73, 0x74, 0x2e,
		0x67, 0x6f, 0xad, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x50,
		0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0xa6, 0x71, 0x73, 0x74,
		0x65, 0x73, 0x74, 0xad, 0x5a, 0x65, 0x62, 0x72, 0x61, 0x53,
		0x63, 0x68, 0x65, 0x6d, 0x61, 0x49, 0x64, 0x00, 0xa7, 0x53,
		0x74, 0x72, 0x75, 0x63, 0x74, 0x73, 0x81, 0xa9, 0x5a, 0x65,
		0x62, 0x72, 0x61, 0x44, 0x61, 0x74, 0x61, 0x82, 0xaa, 0x53,
		0x74, 0x72, 0x75, 0x63, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0xa9,
		0x5a, 0x65, 0x62, 0x72, 0x61, 0x44, 0x61, 0x74, 0x61, 0xa6,
		0x46, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x93, 0x86, 0xa3, 0x5a,
		0x69, 0x64, 0x00, 0xab, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x47,
		0x6f, 0x4e, 0x61, 0x6d, 0x65, 0xa6, 0x55, 0x73, 0x65, 0x72,
		0x69, 0x64, 0xac, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x54, 0x61,
		0x67, 0x4e, 0x61, 0x6d, 0x65, 0xa6, 0x55, 0x73, 0x65, 0x72,
		0x69, 0x64, 0xac, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x54, 0x79,
		0x70, 0x65, 0x53, 0x74, 0x72, 0xa9, 0x5a, 0x65, 0x62, 0x72,
		0x61, 0x55, 0x55, 0x49, 0x44, 0xad, 0x46, 0x69, 0x65, 0x6c,
		0x64, 0x43, 0x61, 0x74, 0x65, 0x67, 0x6f, 0x72, 0x79, 0x1b,
		0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x46, 0x75, 0x6c, 0x6c,
		0x54, 0x79, 0x70, 0x65, 0x84, 0xa4, 0x4b, 0x69, 0x6e, 0x64,
		0x1b, 0xa3, 0x53, 0x74, 0x72, 0xa5, 0x41, 0x72, 0x72, 0x61,
		0x79, 0xa6, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x82, 0xa4,
		0x4b, 0x69, 0x6e, 0x64, 0x10, 0xa3, 0x53, 0x74, 0x72, 0xa2,
		0x31, 0x36, 0xa5, 0x52, 0x61, 0x6e, 0x67, 0x65, 0x82, 0xa4,
		0x4b, 0x69, 0x6e, 0x64, 0x0c, 0xa3, 0x53, 0x74, 0x72, 0xa4,
		0x62, 0x79, 0x74, 0x65, 0x87, 0xa3, 0x5a, 0x69, 0x64, 0x01,
		0xab, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x47, 0x6f, 0x4e, 0x61,
		0x6d, 0x65, 0xa8, 0x55, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d,
		0x65, 0xac, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x54, 0x61, 0x67,
		0x4e, 0x61, 0x6d, 0x65, 0xa8, 0x55, 0x73, 0x65, 0x72, 0x6e,
		0x61, 0x6d, 0x65, 0xac, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x54,
		0x79, 0x70, 0x65, 0x53, 0x74, 0x72, 0xa6, 0x73, 0x74, 0x72,
		0x69, 0x6e, 0x67, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x43,
		0x61, 0x74, 0x65, 0x67, 0x6f, 0x72, 0x79, 0x17, 0xae, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x50, 0x72, 0x69, 0x6d, 0x69, 0x74,
		0x69, 0x76, 0x65, 0x02, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64,
		0x46, 0x75, 0x6c, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x82, 0xa4,
		0x4b, 0x69, 0x6e, 0x64, 0x02, 0xa3, 0x53, 0x74, 0x72, 0xa6,
		0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x87, 0xa3, 0x5a, 0x69,
		0x64, 0x02, 0xab, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x47, 0x6f,
		0x4e, 0x61, 0x6d, 0x65, 0xa4, 0x4e, 0x6f, 0x74, 0x65, 0xac,
		0x46, 0x69, 0x65, 0x6c, 0x64, 0x54, 0x61, 0x67, 0x4e, 0x61,
		0x6d, 0x65, 0xa4, 0x4e, 0x6f, 0x74, 0x65, 0xac, 0x46, 0x69,
		0x65, 0x6c, 0x64, 0x54, 0x79, 0x70, 0x65, 0x53, 0x74, 0x72,
		0xa6, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0xad, 0x46, 0x69,
		0x65, 0x6c, 0x64, 0x43, 0x61, 0x74, 0x65, 0x67, 0x6f, 0x72,
		0x79, 0x17, 0xae, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x50, 0x72,
		0x69, 0x6d, 0x69, 0x74, 0x69, 0x76, 0x65, 0x02, 0xad, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x46, 0x75, 0x6c, 0x6c, 0x54, 0x79,
		0x70, 0x65, 0x82, 0xa4, 0x4b, 0x69, 0x6e, 0x64, 0x02, 0xa3,
		0x53, 0x74, 0x72, 0xa6, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67,
		0xa7, 0x49, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x90,
	}
}

// ZebraSchemaInJsonCompact provides the ZebraPack Schema in compact JSON format, length 686 bytes
func (FileGenerated_zebra_test_go) ZebraSchemaInJsonCompact() []byte {
	return []byte(`{"SourcePath":"bench_zebra_test.go","SourcePackage":"qstest","ZebraSchemaId":0,"Structs":{"ZebraData":{"StructName":"ZebraData","Fields":[{"Zid":0,"FieldGoName":"Userid","FieldTagName":"Userid","FieldTypeStr":"ZebraUUID","FieldCategory":27,"FieldFullType":{"Kind":27,"Str":"Array","Domain":{"Kind":16,"Str":"16"},"Range":{"Kind":12,"Str":"byte"}}},{"Zid":1,"FieldGoName":"Username","FieldTagName":"Username","FieldTypeStr":"string","FieldCategory":23,"FieldPrimitive":2,"FieldFullType":{"Kind":2,"Str":"string"}},{"Zid":2,"FieldGoName":"Note","FieldTagName":"Note","FieldTypeStr":"string","FieldCategory":23,"FieldPrimitive":2,"FieldFullType":{"Kind":2,"Str":"string"}}]}},"Imports":[]}`)
}

// ZebraSchemaInJsonPretty provides the ZebraPack Schema in pretty JSON format, length 1765 bytes
func (FileGenerated_zebra_test_go) ZebraSchemaInJsonPretty() []byte {
	return []byte(`{
    "SourcePath": "bench_zebra_test.go",
    "SourcePackage": "qstest",
    "ZebraSchemaId": 0,
    "Structs": {
        "ZebraData": {
            "StructName": "ZebraData",
            "Fields": [
                {
                    "Zid": 0,
                    "FieldGoName": "Userid",
                    "FieldTagName": "Userid",
                    "FieldTypeStr": "ZebraUUID",
                    "FieldCategory": 27,
                    "FieldFullType": {
                        "Kind": 27,
                        "Str": "Array",
                        "Domain": {
                            "Kind": 16,
                            "Str": "16"
                        },
                        "Range": {
                            "Kind": 12,
                            "Str": "byte"
                        }
                    }
                },
                {
                    "Zid": 1,
                    "FieldGoName": "Username",
                    "FieldTagName": "Username",
                    "FieldTypeStr": "string",
                    "FieldCategory": 23,
                    "FieldPrimitive": 2,
                    "FieldFullType": {
                        "Kind": 2,
                        "Str": "string"
                    }
                },
                {
                    "Zid": 2,
                    "FieldGoName": "Note",
                    "FieldTagName": "Note",
                    "FieldTypeStr": "string",
                    "FieldCategory": 23,
                    "FieldPrimitive": 2,
                    "FieldFullType": {
                        "Kind": 2,
                        "Str": "string"
                    }
                }
            ]
        }
    },
    "Imports": []
}`)
}
