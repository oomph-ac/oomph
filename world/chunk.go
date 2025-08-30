package world

import (
	"bytes"

	"github.com/cespare/xxhash/v2"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func FetchChunkFooterAndBlobs(
	chunkDat []byte,
	airRID uint32,
	subChunkCount int,
	dim world.Dimension,
) ([]protocol.CacheBlob, []byte, error) {
	buf := bytes.NewBuffer(chunkDat)
	_, blobs, err := chunk.NetworkDecodeBuffer(
		airRID,
		buf,
		subChunkCount,
		dim.Range(),
	)
	if err != nil {
		return nil, nil, err
	}

	cBlobs := make([]protocol.CacheBlob, len(blobs))
	for index, blob := range blobs {
		cBlobs[index] = protocol.CacheBlob{
			Payload: blob,
			Hash:    xxhash.Sum64(blob),
		}
	}
	return cBlobs, buf.Bytes(), nil
}
