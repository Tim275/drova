package schema

import (
	"encoding/binary"
	"fmt"

	"github.com/hamba/avro/v2"
)

const magicByte = 0x00

// Encode serialises v using schema and prepends the Confluent wire-format header:
//
//	[0x00][4-byte big-endian schemaID][avro bytes]
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

// Decode reads the Confluent wire-format header, verifies the magic byte,
// and decodes the Avro payload into v using the provided schema.
// The schema must match the one used to encode the message.
func Decode(data []byte, schema avro.Schema, v any) error {
	if len(data) < 5 {
		return fmt.Errorf("message too short to contain schema registry header")
	}
	if data[0] != magicByte {
		return fmt.Errorf("invalid magic byte 0x%02x (expected 0x00)", data[0])
	}
	return avro.Unmarshal(schema, data[5:], v)
}

// SchemaIDFrom reads only the 4-byte schema ID from a wire-format message
// without decoding the payload. Useful when the consumer needs to look up
// the schema dynamically via RegistryClient.GetByID.
func SchemaIDFrom(data []byte) (int, error) {
	if len(data) < 5 {
		return 0, fmt.Errorf("message too short")
	}
	if data[0] != magicByte {
		return 0, fmt.Errorf("invalid magic byte")
	}
	return int(binary.BigEndian.Uint32(data[1:5])), nil
}

// MustParseSchema parses an Avro schema string and panics on error.
// Intended for package-level schema variable initialisation.
func MustParseSchema(raw string) avro.Schema {
	s, err := avro.Parse(raw)
	if err != nil {
		panic("avro schema parse: " + err.Error())
	}
	return s
}

// SchemaFor is a convenience wrapper that reads a .avsc file embedded via
// go:embed and returns a parsed avro.Schema ready for use with Encode/Decode.
func SchemaFor(raw string) (avro.Schema, error) {
	return avro.Parse(raw)
}

// IsWireFormat returns true if data starts with the Confluent magic byte,
// allowing consumers to distinguish Avro messages from plain JSON messages
// during a progressive migration.
func IsWireFormat(data []byte) bool {
	return len(data) >= 5 && data[0] == magicByte
}
