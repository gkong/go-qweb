package bench

// NOTE: THIS FILE WAS PRODUCED BY THE
// ZEBRAPACK CODE GENERATION TOOL (github.com/glycerine/zebrapack)
// DO NOT EDIT

import "github.com/glycerine/zebrapack/msgp"

// DecodeMsg implements msgp.Decodable
// We treat empty fields as if we read a Nil from the wire.
func (z *BenchSessVal) DecodeMsg(dc *msgp.Reader) (err error) {
	var sawTopNil bool
	if dc.IsNil() {
		sawTopNil = true
		err = dc.ReadNil()
		if err != nil {
			return
		}
		dc.PushAlwaysNil()
	}

	var field []byte
	_ = field
	const maxFields0zjph = 4

	// -- templateDecodeMsgZid starts here--
	var totalEncodedFields0zjph uint32
	totalEncodedFields0zjph, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	encodedFieldsLeft0zjph := totalEncodedFields0zjph
	missingFieldsLeft0zjph := maxFields0zjph - totalEncodedFields0zjph

	var nextMiss0zjph int = -1
	var found0zjph [maxFields0zjph]bool
	var curField0zjph int

doneWithStruct0zjph:
	// First fill all the encoded fields, then
	// treat the remaining, missing fields, as Nil.
	for encodedFieldsLeft0zjph > 0 || missingFieldsLeft0zjph > 0 {
		//fmt.Printf("encodedFieldsLeft: %v, missingFieldsLeft: %v, found: '%v', fields: '%#v'\n", encodedFieldsLeft0zjph, missingFieldsLeft0zjph, msgp.ShowFound(found0zjph[:]), decodeMsgFieldOrder0zjph)
		if encodedFieldsLeft0zjph > 0 {
			encodedFieldsLeft0zjph--
			curField0zjph, err = dc.ReadInt()
			if err != nil {
				return
			}
		} else {
			//missing fields need handling
			if nextMiss0zjph < 0 {
				// tell the reader to only give us Nils
				// until further notice.
				dc.PushAlwaysNil()
				nextMiss0zjph = 0
			}
			for nextMiss0zjph < maxFields0zjph && (found0zjph[nextMiss0zjph] || decodeMsgFieldSkip0zjph[nextMiss0zjph]) {
				nextMiss0zjph++
			}
			if nextMiss0zjph == maxFields0zjph {
				// filled all the empty fields!
				break doneWithStruct0zjph
			}
			missingFieldsLeft0zjph--
			curField0zjph = nextMiss0zjph
		}
		//fmt.Printf("switching on curField: '%v'\n", curField0zjph)
		switch curField0zjph {
		// -- templateDecodeMsgZid ends here --

		case 0:
			// zid 0 for "Expiration"
			found0zjph[0] = true
			z.Expiration, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case 1:
			// zid 1 for "MaxAge"
			found0zjph[1] = true
			z.MaxAge, err = dc.ReadInt()
			if err != nil {
				return
			}
		case 2:
			// zid 2 for "MinRefresh"
			found0zjph[2] = true
			z.MinRefresh, err = dc.ReadInt()
			if err != nil {
				return
			}
		case 3:
			// zid 3 for "Data"
			found0zjph[3] = true
			z.Data, err = dc.ReadBytes(z.Data)
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
	if nextMiss0zjph != -1 {
		dc.PopAlwaysNil()
	}

	if sawTopNil {
		dc.PopAlwaysNil()
	}

	if p, ok := interface{}(z).(msgp.PostLoad); ok {
		p.PostLoadHook()
	}

	return
}

// fields of BenchSessVal
var decodeMsgFieldOrder0zjph = []string{"Expiration", "MaxAge", "MinRefresh", "Data"}

var decodeMsgFieldSkip0zjph = []bool{false, false, false, false}

// fieldsNotEmpty supports omitempty tags
func (z *BenchSessVal) fieldsNotEmpty(isempty []bool) uint32 {
	if len(isempty) == 0 {
		return 4
	}
	var fieldsInUse uint32 = 4
	isempty[0] = (z.Expiration == 0) // number, omitempty
	if isempty[0] {
		fieldsInUse--
	}
	isempty[1] = (z.MaxAge == 0) // number, omitempty
	if isempty[1] {
		fieldsInUse--
	}
	isempty[2] = (z.MinRefresh == 0) // number, omitempty
	if isempty[2] {
		fieldsInUse--
	}
	isempty[3] = (len(z.Data) == 0) // string, omitempty
	if isempty[3] {
		fieldsInUse--
	}

	return fieldsInUse
}

// EncodeMsg implements msgp.Encodable
func (z *BenchSessVal) EncodeMsg(en *msgp.Writer) (err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	// honor the omitempty tags
	var empty_zqpy [4]bool
	fieldsInUse_zdur := z.fieldsNotEmpty(empty_zqpy[:])

	// map header
	err = en.WriteMapHeader(fieldsInUse_zdur)
	if err != nil {
		return err
	}

	if !empty_zqpy[0] {
		// zid 0 for "Expiration"
		err = en.Append(0x0)
		if err != nil {
			return err
		}
		err = en.WriteInt64(z.Expiration)
		if err != nil {
			return
		}
	}

	if !empty_zqpy[1] {
		// zid 1 for "MaxAge"
		err = en.Append(0x1)
		if err != nil {
			return err
		}
		err = en.WriteInt(z.MaxAge)
		if err != nil {
			return
		}
	}

	if !empty_zqpy[2] {
		// zid 2 for "MinRefresh"
		err = en.Append(0x2)
		if err != nil {
			return err
		}
		err = en.WriteInt(z.MinRefresh)
		if err != nil {
			return
		}
	}

	if !empty_zqpy[3] {
		// zid 3 for "Data"
		err = en.Append(0x3)
		if err != nil {
			return err
		}
		err = en.WriteBytes(z.Data)
		if err != nil {
			return
		}
	}

	return
}

// MarshalMsg implements msgp.Marshaler
func (z *BenchSessVal) MarshalMsg(b []byte) (o []byte, err error) {
	if p, ok := interface{}(z).(msgp.PreSave); ok {
		p.PreSaveHook()
	}

	o = msgp.Require(b, z.Msgsize())

	// honor the omitempty tags
	var empty [4]bool
	fieldsInUse := z.fieldsNotEmpty(empty[:])
	o = msgp.AppendMapHeader(o, fieldsInUse)

	if !empty[0] {
		// zid 0 for "Expiration"
		o = append(o, 0x0)
		o = msgp.AppendInt64(o, z.Expiration)
	}

	if !empty[1] {
		// zid 1 for "MaxAge"
		o = append(o, 0x1)
		o = msgp.AppendInt(o, z.MaxAge)
	}

	if !empty[2] {
		// zid 2 for "MinRefresh"
		o = append(o, 0x2)
		o = msgp.AppendInt(o, z.MinRefresh)
	}

	if !empty[3] {
		// zid 3 for "Data"
		o = append(o, 0x3)
		o = msgp.AppendBytes(o, z.Data)
	}

	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *BenchSessVal) UnmarshalMsg(bts []byte) (o []byte, err error) {
	return z.UnmarshalMsgWithCfg(bts, nil)
}
func (z *BenchSessVal) UnmarshalMsgWithCfg(bts []byte, cfg *msgp.RuntimeConfig) (o []byte, err error) {
	var nbs msgp.NilBitsStack
	nbs.Init(cfg)
	var sawTopNil bool
	if msgp.IsNil(bts) {
		sawTopNil = true
		bts = nbs.PushAlwaysNil(bts[1:])
	}

	var field []byte
	_ = field
	const maxFields1zbsa = 4

	// -- templateUnmarshalMsgZid starts here--
	var totalEncodedFields1zbsa uint32
	if !nbs.AlwaysNil {
		totalEncodedFields1zbsa, bts, err = nbs.ReadMapHeaderBytes(bts)
		if err != nil {
			return
		}
	}
	encodedFieldsLeft1zbsa := totalEncodedFields1zbsa
	missingFieldsLeft1zbsa := maxFields1zbsa - totalEncodedFields1zbsa

	var nextMiss1zbsa int = -1
	var found1zbsa [maxFields1zbsa]bool
	var curField1zbsa int

doneWithStruct1zbsa:
	// First fill all the encoded fields, then
	// treat the remaining, missing fields, as Nil.
	for encodedFieldsLeft1zbsa > 0 || missingFieldsLeft1zbsa > 0 {
		//fmt.Printf("encodedFieldsLeft: %v, missingFieldsLeft: %v, found: '%v', fields: '%#v'\n", encodedFieldsLeft1zbsa, missingFieldsLeft1zbsa, msgp.ShowFound(found1zbsa[:]), unmarshalMsgFieldOrder1zbsa)
		if encodedFieldsLeft1zbsa > 0 {
			encodedFieldsLeft1zbsa--
			curField1zbsa, bts, err = nbs.ReadIntBytes(bts)
			if err != nil {
				return
			}
		} else {
			//missing fields need handling
			if nextMiss1zbsa < 0 {
				// set bts to contain just mnil (0xc0)
				bts = nbs.PushAlwaysNil(bts)
				nextMiss1zbsa = 0
			}
			for nextMiss1zbsa < maxFields1zbsa && (found1zbsa[nextMiss1zbsa] || unmarshalMsgFieldSkip1zbsa[nextMiss1zbsa]) {
				nextMiss1zbsa++
			}
			if nextMiss1zbsa == maxFields1zbsa {
				// filled all the empty fields!
				break doneWithStruct1zbsa
			}
			missingFieldsLeft1zbsa--
			curField1zbsa = nextMiss1zbsa
		}
		//fmt.Printf("switching on curField: '%v'\n", curField1zbsa)
		switch curField1zbsa {
		// -- templateUnmarshalMsgZid ends here --

		case 0:
			// zid 0 for "Expiration"
			found1zbsa[0] = true
			z.Expiration, bts, err = nbs.ReadInt64Bytes(bts)

			if err != nil {
				return
			}
		case 1:
			// zid 1 for "MaxAge"
			found1zbsa[1] = true
			z.MaxAge, bts, err = nbs.ReadIntBytes(bts)

			if err != nil {
				return
			}
		case 2:
			// zid 2 for "MinRefresh"
			found1zbsa[2] = true
			z.MinRefresh, bts, err = nbs.ReadIntBytes(bts)

			if err != nil {
				return
			}
		case 3:
			// zid 3 for "Data"
			found1zbsa[3] = true
			if nbs.AlwaysNil || msgp.IsNil(bts) {
				if !nbs.AlwaysNil {
					bts = bts[1:]
				}
				z.Data = z.Data[:0]
			} else {
				z.Data, bts, err = nbs.ReadBytesBytes(bts, z.Data)

				if err != nil {
					return
				}
			}
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
	if nextMiss1zbsa != -1 {
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

// fields of BenchSessVal
var unmarshalMsgFieldOrder1zbsa = []string{"Expiration", "MaxAge", "MinRefresh", "Data"}

var unmarshalMsgFieldSkip1zbsa = []bool{false, false, false, false}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *BenchSessVal) Msgsize() (s int) {
	s = 1 + 11 + msgp.Int64Size + 7 + msgp.IntSize + 11 + msgp.IntSize + 5 + msgp.BytesPrefixSize + len(z.Data)
	return
}

// FileSchemaData_generated_go holds ZebraPack schema from file 'schemaData.go'
type FileSchemaData_generated_go struct{}

// ZebraSchemaInMsgpack2Format provides the ZebraPack Schema in msgpack2 format, length 634 bytes
func (FileSchemaData_generated_go) ZebraSchemaInMsgpack2Format() []byte {
	return []byte{
		0x85, 0xaa, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x50, 0x61,
		0x74, 0x68, 0xad, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x44,
		0x61, 0x74, 0x61, 0x2e, 0x67, 0x6f, 0xad, 0x53, 0x6f, 0x75,
		0x72, 0x63, 0x65, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65,
		0xa5, 0x62, 0x65, 0x6e, 0x63, 0x68, 0xad, 0x5a, 0x65, 0x62,
		0x72, 0x61, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x49, 0x64,
		0x00, 0xa7, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x73, 0x81,
		0xac, 0x42, 0x65, 0x6e, 0x63, 0x68, 0x53, 0x65, 0x73, 0x73,
		0x56, 0x61, 0x6c, 0x82, 0xaa, 0x53, 0x74, 0x72, 0x75, 0x63,
		0x74, 0x4e, 0x61, 0x6d, 0x65, 0xac, 0x42, 0x65, 0x6e, 0x63,
		0x68, 0x53, 0x65, 0x73, 0x73, 0x56, 0x61, 0x6c, 0xa6, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x73, 0x94, 0x87, 0xa3, 0x5a, 0x69,
		0x64, 0x00, 0xab, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x47, 0x6f,
		0x4e, 0x61, 0x6d, 0x65, 0xaa, 0x45, 0x78, 0x70, 0x69, 0x72,
		0x61, 0x74, 0x69, 0x6f, 0x6e, 0xac, 0x46, 0x69, 0x65, 0x6c,
		0x64, 0x54, 0x61, 0x67, 0x4e, 0x61, 0x6d, 0x65, 0xaa, 0x45,
		0x78, 0x70, 0x69, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0xac,
		0x46, 0x69, 0x65, 0x6c, 0x64, 0x54, 0x79, 0x70, 0x65, 0x53,
		0x74, 0x72, 0xa5, 0x69, 0x6e, 0x74, 0x36, 0x34, 0xad, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x43, 0x61, 0x74, 0x65, 0x67, 0x6f,
		0x72, 0x79, 0x17, 0xae, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x50,
		0x72, 0x69, 0x6d, 0x69, 0x74, 0x69, 0x76, 0x65, 0x11, 0xad,
		0x46, 0x69, 0x65, 0x6c, 0x64, 0x46, 0x75, 0x6c, 0x6c, 0x54,
		0x79, 0x70, 0x65, 0x82, 0xa4, 0x4b, 0x69, 0x6e, 0x64, 0x11,
		0xa3, 0x53, 0x74, 0x72, 0xa5, 0x69, 0x6e, 0x74, 0x36, 0x34,
		0x87, 0xa3, 0x5a, 0x69, 0x64, 0x01, 0xab, 0x46, 0x69, 0x65,
		0x6c, 0x64, 0x47, 0x6f, 0x4e, 0x61, 0x6d, 0x65, 0xa6, 0x4d,
		0x61, 0x78, 0x41, 0x67, 0x65, 0xac, 0x46, 0x69, 0x65, 0x6c,
		0x64, 0x54, 0x61, 0x67, 0x4e, 0x61, 0x6d, 0x65, 0xa6, 0x4d,
		0x61, 0x78, 0x41, 0x67, 0x65, 0xac, 0x46, 0x69, 0x65, 0x6c,
		0x64, 0x54, 0x79, 0x70, 0x65, 0x53, 0x74, 0x72, 0xa3, 0x69,
		0x6e, 0x74, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x43, 0x61,
		0x74, 0x65, 0x67, 0x6f, 0x72, 0x79, 0x17, 0xae, 0x46, 0x69,
		0x65, 0x6c, 0x64, 0x50, 0x72, 0x69, 0x6d, 0x69, 0x74, 0x69,
		0x76, 0x65, 0x0d, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x46,
		0x75, 0x6c, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x82, 0xa4, 0x4b,
		0x69, 0x6e, 0x64, 0x0d, 0xa3, 0x53, 0x74, 0x72, 0xa3, 0x69,
		0x6e, 0x74, 0x87, 0xa3, 0x5a, 0x69, 0x64, 0x02, 0xab, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x47, 0x6f, 0x4e, 0x61, 0x6d, 0x65,
		0xaa, 0x4d, 0x69, 0x6e, 0x52, 0x65, 0x66, 0x72, 0x65, 0x73,
		0x68, 0xac, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x54, 0x61, 0x67,
		0x4e, 0x61, 0x6d, 0x65, 0xaa, 0x4d, 0x69, 0x6e, 0x52, 0x65,
		0x66, 0x72, 0x65, 0x73, 0x68, 0xac, 0x46, 0x69, 0x65, 0x6c,
		0x64, 0x54, 0x79, 0x70, 0x65, 0x53, 0x74, 0x72, 0xa3, 0x69,
		0x6e, 0x74, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x43, 0x61,
		0x74, 0x65, 0x67, 0x6f, 0x72, 0x79, 0x17, 0xae, 0x46, 0x69,
		0x65, 0x6c, 0x64, 0x50, 0x72, 0x69, 0x6d, 0x69, 0x74, 0x69,
		0x76, 0x65, 0x0d, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x46,
		0x75, 0x6c, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x82, 0xa4, 0x4b,
		0x69, 0x6e, 0x64, 0x0d, 0xa3, 0x53, 0x74, 0x72, 0xa3, 0x69,
		0x6e, 0x74, 0x87, 0xa3, 0x5a, 0x69, 0x64, 0x03, 0xab, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x47, 0x6f, 0x4e, 0x61, 0x6d, 0x65,
		0xa4, 0x44, 0x61, 0x74, 0x61, 0xac, 0x46, 0x69, 0x65, 0x6c,
		0x64, 0x54, 0x61, 0x67, 0x4e, 0x61, 0x6d, 0x65, 0xa4, 0x44,
		0x61, 0x74, 0x61, 0xac, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x54,
		0x79, 0x70, 0x65, 0x53, 0x74, 0x72, 0xa6, 0x5b, 0x5d, 0x62,
		0x79, 0x74, 0x65, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x43,
		0x61, 0x74, 0x65, 0x67, 0x6f, 0x72, 0x79, 0x17, 0xae, 0x46,
		0x69, 0x65, 0x6c, 0x64, 0x50, 0x72, 0x69, 0x6d, 0x69, 0x74,
		0x69, 0x76, 0x65, 0x01, 0xad, 0x46, 0x69, 0x65, 0x6c, 0x64,
		0x46, 0x75, 0x6c, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x82, 0xa4,
		0x4b, 0x69, 0x6e, 0x64, 0x01, 0xa3, 0x53, 0x74, 0x72, 0xa5,
		0x62, 0x79, 0x74, 0x65, 0x73, 0xa7, 0x49, 0x6d, 0x70, 0x6f,
		0x72, 0x74, 0x73, 0x90,
	}
}

// ZebraSchemaInJsonCompact provides the ZebraPack Schema in compact JSON format, length 800 bytes
func (FileSchemaData_generated_go) ZebraSchemaInJsonCompact() []byte {
	return []byte(`{"SourcePath":"schemaData.go","SourcePackage":"bench","ZebraSchemaId":0,"Structs":{"BenchSessVal":{"StructName":"BenchSessVal","Fields":[{"Zid":0,"FieldGoName":"Expiration","FieldTagName":"Expiration","FieldTypeStr":"int64","FieldCategory":23,"FieldPrimitive":17,"FieldFullType":{"Kind":17,"Str":"int64"}},{"Zid":1,"FieldGoName":"MaxAge","FieldTagName":"MaxAge","FieldTypeStr":"int","FieldCategory":23,"FieldPrimitive":13,"FieldFullType":{"Kind":13,"Str":"int"}},{"Zid":2,"FieldGoName":"MinRefresh","FieldTagName":"MinRefresh","FieldTypeStr":"int","FieldCategory":23,"FieldPrimitive":13,"FieldFullType":{"Kind":13,"Str":"int"}},{"Zid":3,"FieldGoName":"Data","FieldTagName":"Data","FieldTypeStr":"[]byte","FieldCategory":23,"FieldPrimitive":1,"FieldFullType":{"Kind":1,"Str":"bytes"}}]}},"Imports":[]}`)
}

// ZebraSchemaInJsonPretty provides the ZebraPack Schema in pretty JSON format, length 1940 bytes
func (FileSchemaData_generated_go) ZebraSchemaInJsonPretty() []byte {
	return []byte(`{
    "SourcePath": "schemaData.go",
    "SourcePackage": "bench",
    "ZebraSchemaId": 0,
    "Structs": {
        "BenchSessVal": {
            "StructName": "BenchSessVal",
            "Fields": [
                {
                    "Zid": 0,
                    "FieldGoName": "Expiration",
                    "FieldTagName": "Expiration",
                    "FieldTypeStr": "int64",
                    "FieldCategory": 23,
                    "FieldPrimitive": 17,
                    "FieldFullType": {
                        "Kind": 17,
                        "Str": "int64"
                    }
                },
                {
                    "Zid": 1,
                    "FieldGoName": "MaxAge",
                    "FieldTagName": "MaxAge",
                    "FieldTypeStr": "int",
                    "FieldCategory": 23,
                    "FieldPrimitive": 13,
                    "FieldFullType": {
                        "Kind": 13,
                        "Str": "int"
                    }
                },
                {
                    "Zid": 2,
                    "FieldGoName": "MinRefresh",
                    "FieldTagName": "MinRefresh",
                    "FieldTypeStr": "int",
                    "FieldCategory": 23,
                    "FieldPrimitive": 13,
                    "FieldFullType": {
                        "Kind": 13,
                        "Str": "int"
                    }
                },
                {
                    "Zid": 3,
                    "FieldGoName": "Data",
                    "FieldTagName": "Data",
                    "FieldTypeStr": "[]byte",
                    "FieldCategory": 23,
                    "FieldPrimitive": 1,
                    "FieldFullType": {
                        "Kind": 1,
                        "Str": "bytes"
                    }
                }
            ]
        }
    },
    "Imports": []
}`)
}
