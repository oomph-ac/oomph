package blocknetwork

import "github.com/df-mc/dragonfly/server/world/chunk"

// Codec converts IDs used by one network endpoint to and from canonical block-registry runtime IDs.
type Codec struct {
	registry chunk.BlockRegistry
	mode     Mode
}

// NewCodec returns a Codec backed by registry using mode. It panics if registry is nil.
func NewCodec(registry chunk.BlockRegistry, mode Mode) Codec {
	if registry == nil {
		panic("blocknetwork: nil block registry")
	}
	return Codec{registry: registry, mode: mode}
}

// Mode returns the network representation used by the codec.
func (c Codec) Mode() Mode {
	return c.mode
}

// ToRuntimeID converts a network ID to a canonical block-registry runtime ID.
func (c Codec) ToRuntimeID(networkID uint32) (uint32, bool) {
	if c.mode == Hashes {
		return c.registry.HashToRuntimeID(networkID)
	}
	_, _, ok := c.registry.RuntimeIDToState(networkID)
	return networkID, ok
}

// FromRuntimeID converts a canonical block-registry runtime ID to the codec's network representation.
func (c Codec) FromRuntimeID(runtimeID uint32) (uint32, bool) {
	if c.mode == Hashes {
		return c.registry.RuntimeIDToHash(runtimeID)
	}
	_, _, ok := c.registry.RuntimeIDToState(runtimeID)
	return runtimeID, ok
}

// Translator converts block IDs from one endpoint codec to another.
type Translator struct {
	source Codec
	target Codec
}

// NewTranslator returns a Translator that converts IDs from source to target.
func NewTranslator(source, target Codec) Translator {
	return Translator{source: source, target: target}
}

// Required reports whether source and target use different block ID representations.
func (t Translator) Required() bool {
	return t.source.Mode() != t.target.Mode()
}

// Translate converts a source network ID to the target representation. Unknown IDs are preserved.
func (t Translator) Translate(networkID uint32) uint32 {
	if !t.Required() {
		return networkID
	}
	runtimeID, ok := t.source.ToRuntimeID(networkID)
	if !ok {
		return networkID
	}
	translated, ok := t.target.FromRuntimeID(runtimeID)
	if !ok {
		return networkID
	}
	return translated
}
