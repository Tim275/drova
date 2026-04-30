package schema

import (
	"encoding/binary"
	"fmt"

	"github.com/hamba/avro/v2"
)

const magicByte = 0x00

func Encode(schemaID int, schema avro.Schema, v any) ([]byte, error) {
	avroBytes, err := avro.Marshal(schema, v)
	if err != nil {
		return nil, fmt.Errorf("avro marshal: %w", err)
	}

	buf := make([]byte, 5, 5+len(avroBytes))
	buf[0] = magicByte
	binary.BigEndian.PutUint32(buf[1:5], uint32(schemaID))
	return append(buf, avroBytes...), nil
}

func Decode(data []byte, schema avro.Schema, v any) error {
	if len(data) < 5 {
		return fmt.Errorf("message too short to contain schema registry header")
	}
	if data[0] != magicByte {
		return fmt.Errorf("invalid magic byte 0x%02x (expected 0x00)", data[0])
	}
	return avro.Unmarshal(schema, data[5:], v)
}

func SchemaIDFrom(data []byte) (int, error) {
	if len(data) < 5 {
		return 0, fmt.Errorf("message too short")
	}
	if data[0] != magicByte {
		return 0, fmt.Errorf("invalid magic byte")
	}
	return int(binary.BigEndian.Uint32(data[1:5])), nil
}

func MustParseSchema(raw string) avro.Schema {
	s, err := avro.Parse(raw)
	if err != nil {
		panic("avro schema parse: " + err.Error())
	}
	return s
}

func SchemaFor(raw string) (avro.Schema, error) {
	return avro.Parse(raw)
}

func IsWireFormat(data []byte) bool {
	return len(data) >= 5 && data[0] == magicByte
}
