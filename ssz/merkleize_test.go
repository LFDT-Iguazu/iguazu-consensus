// Copyright 2026 Digital Clever Solution LLC
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
	"testing"

	"github.com/stretchr/testify/require"
)

// TestZeroHashesMatchPrysmTable pins the first three entries to the
// values in the coordinator's container/trie.ZeroHashes table. This is
// the load-bearing compatibility check: if these ever drift, consensus
// roots diverge from the existing coordinator.
func TestZeroHashesMatchPrysmTable(t *testing.T) {
	require.Equal(t, [32]byte{}, zeroHashes[0])
	require.Equal(t, [32]byte{
		245, 165, 253, 66, 209, 106, 32, 48, 39, 152, 239, 110, 211, 9, 151, 155,
		67, 0, 61, 35, 32, 217, 240, 232, 234, 152, 49, 169, 39, 89, 251, 75,
	}, zeroHashes[1])
	require.Equal(t, [32]byte{
		219, 86, 17, 78, 0, 253, 212, 193, 248, 92, 137, 43, 243, 90, 201, 168,
		146, 137, 170, 236, 177, 235, 208, 169, 108, 222, 96, 106, 116, 139, 93, 113,
	}, zeroHashes[2])
}

// TestZeroHashesMatchSpec verifies our zero-hash table against the
// recursive SSZ definition, so that any future refactor cannot silently
// desync from other implementations.
func TestZeroHashesMatchSpec(t *testing.T) {
	var prev [32]byte
	for d := 0; d < len(zeroHashes); d++ {
		require.Equal(t, prev, zeroHashes[d], "zeroHashes[%d]", d)
		var buf [64]byte
		copy(buf[:32], prev[:])
		copy(buf[32:], prev[:])
		prev = sha256.Sum256(buf[:])
	}
}

func TestDepth(t *testing.T) {
	cases := []struct {
		in  uint64
		out uint8
	}{
		{0, 0}, {1, 0}, {2, 1}, {3, 2}, {4, 2}, {5, 3}, {8, 3}, {9, 4},
	}
	for _, c := range cases {
		require.Equal(t, c.out, Depth(c.in), "Depth(%d)", c.in)
	}
}

func TestBitwiseMerkleizeSingleChunk(t *testing.T) {
	// Single chunk with limit 1 is the identity.
	chunk := ToBytes32([]byte("hello"))
	root, err := BitwiseMerkleize([][32]byte{chunk}, 1, 1)
	require.NoError(t, err)
	require.Equal(t, chunk, root)
}

func TestBitwiseMerkleizeOverLimit(t *testing.T) {
	_, err := BitwiseMerkleize([][32]byte{{}, {}}, 2, 1)
	require.Error(t, err)
}

func TestBitwiseMerkleizeEmpty(t *testing.T) {
	root, err := BitwiseMerkleize(nil, 0, 4)
	require.NoError(t, err)
	// Empty list with limit 4 yields zeroHashes at Depth(4)=2.
	require.Equal(t, zeroHashes[2], root)
}

func TestMixInLengthDeterministic(t *testing.T) {
	r := ToBytes32([]byte("root"))
	a := MixInLength(r, Uint64LengthRoot(7))
	b := MixInLength(r, Uint64LengthRoot(7))
	require.Equal(t, a, b)
	c := MixInLength(r, Uint64LengthRoot(8))
	require.NotEqual(t, a, c)
}
