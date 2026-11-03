package grpcjson

import "encoding/json"

// Codec keeps the internal RPC contract readable while allowing the service
// to run without a protoc toolchain in development containers.
type Codec struct{}

func (Codec) Name() string { return "json" }

func (Codec) Marshal(value any) ([]byte, error) { return json.Marshal(value) }

func (Codec) Unmarshal(payload []byte, target any) error { return json.Unmarshal(payload, target) }
