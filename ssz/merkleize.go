// Copyright 2026 Digital Clever Solution LLC
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssz

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/bits"
)

// BytesPerChunk is the SSZ spec chunk size.
const BytesPerChunk = 32

// zeroHashes holds sha256 zero-hash values at successive depths.
//
//	zeroHashes[0] = [32]byte{}
//	zeroHashes[d] = sha256(zeroHashes[d-1] || zeroHashes[d-1])
//
// 64 entries cover every tree we realistically Merkleize.
var zeroHashes [64][32]byte

func init() {
	var buf [64]byte
	for d := 1; d < len(zeroHashes); d++ {
		copy(buf[:32], zeroHashes[d-1][:])
		copy(buf[32:], zeroHashes[d-1][:])
		zeroHashes[d] = sha256.Sum256(buf[:])
	}
}

// Depth returns the number of levels required to Merkleize v leaves, i.e.
// the smallest d such that (1 << d) >= v. Depth(0) and Depth(1) are 0.
func Depth(v uint64) uint8 {
	if v <= 1 {
		return 0
	}
	return uint8(bits.Len64(v - 1))
}

// hashPair returns sha256(left || right).
func hashPair(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:32], left[:])
	copy(buf[32:], right[:])
	return sha256.Sum256(buf[:])
}

// BitwiseMerkleize Merkleizes count leaves drawn from chunks, padding with
// zero hashes up to the next power of two and then further up to limit.
// It returns an error if count > limit.
func BitwiseMerkleize(chunks [][32]byte, count, limit uint64) ([32]byte, error) {
	if count > limit {
		return [32]byte{}, errors.New("ssz: merkleizing list that is too large, over limit")
	}
	return MerkleizeVector(chunks, limit), nil
}

// MerkleizeVector Merkleizes the given elements as leaves of a virtual
// balanced binary tree with (1 << Depth(length)) leaves. Missing leaves are
// filled with zero hashes at the appropriate depth. The result is the root.
func MerkleizeVector(elements [][32]byte, length uint64) [32]byte {
	depth := Depth(length)
	if len(elements) == 0 {
		return zeroHashes[depth]
	}

	layer := make([][32]byte, len(elements))
	copy(layer, elements)

	for d := uint8(0); d < depth; d++ {
		if len(layer)%2 == 1 {
			layer = append(layer, zeroHashes[d])
		}
		next := make([][32]byte, len(layer)/2)
		for i := 0; i < len(next); i++ {
			next[i] = hashPair(layer[2*i], layer[2*i+1])
		}
		layer = next
	}
	return layer[0]
}

// MixInLength returns sha256(root || length). The length slice is expected
// to already be 32 bytes with the uint64 little-endian value in its lowest
// eight bytes.
func MixInLength(root [32]byte, length []byte) [32]byte {
	var buf [64]byte
	copy(buf[:32], root[:])
	copy(buf[32:], length)
	return sha256.Sum256(buf[:])
}

// Uint64LengthRoot returns a 32-byte buffer whose first 8 bytes are the
// little-endian encoding of n; suitable for use as the second argument of
// MixInLength.
func Uint64LengthRoot(n uint64) []byte {
	out := make([]byte, 32)
	binary.LittleEndian.PutUint64(out[:8], n)
	return out
}

// PackByChunk packs the concatenation of serializedItems into 32-byte
// chunks, right-padding the last chunk with zeros. An empty input yields a
// single zero chunk — matching the SSZ spec for merkleizing an empty leaf
// list.
func PackByChunk(serializedItems [][]byte) ([][32]byte, error) {
	if len(serializedItems) == 0 {
		return [][32]byte{{}}, nil
	}
	total := 0
	for _, item := range serializedItems {
		total += len(item)
	}
	if total == 0 {
		return [][32]byte{{}}, nil
	}

	flat := make([]byte, 0, total)
	for _, item := range serializedItems {
		flat = append(flat, item...)
	}

	n := (len(flat) + BytesPerChunk - 1) / BytesPerChunk
	chunks := make([][32]byte, n)
	for i := 0; i < n; i++ {
		from := i * BytesPerChunk
		to := from + BytesPerChunk
		if to > len(flat) {
			to = len(flat)
		}
		copy(chunks[i][:], flat[from:to])
	}
	return chunks, nil
}

// ToBytes32 right-pads (or truncates) b into a 32-byte array.
func ToBytes32(b []byte) [32]byte {
	var out [32]byte
	copy(out[:], b)
	return out
}
