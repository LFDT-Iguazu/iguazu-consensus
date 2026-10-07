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

package spine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

func TestDataRoot_Nil(t *testing.T) {
	_, err := DataRoot(nil)
	require.Error(t, err)
}

func TestDataRoot_DeterministicAndDiverges(t *testing.T) {
	sd := &types.SpineData{
		Spines:       []byte{0x01, 0x02, 0x03},
		Prefix:       []byte{0xaa},
		Finalization: []byte{0xbb, 0xcc},
		CpFinalized:  []byte{0xdd},
		ParentSpines: []*types.SpinesSeq{
			{Spines: []byte{0x11, 0x22}},
			{Spines: []byte{}},
		},
	}
	r1, err := DataRoot(sd)
	require.NoError(t, err)
	r2, err := DataRoot(sd)
	require.NoError(t, err)
	require.Equal(t, r1, r2)

	// Flip a single byte and make sure the root changes.
	sd2 := &types.SpineData{
		Spines:       []byte{0x01, 0x02, 0x04},
		Prefix:       []byte{0xaa},
		Finalization: []byte{0xbb, 0xcc},
		CpFinalized:  []byte{0xdd},
		ParentSpines: sd.ParentSpines,
	}
	r3, err := DataRoot(sd2)
	require.NoError(t, err)
	require.NotEqual(t, r1, r3)
}

func TestBytesRoot_EmptyStable(t *testing.T) {
	r, err := BytesRoot(nil)
	require.NoError(t, err)
	// Empty bytes list: chunks=0, length=0 — MixInLength of empty chunksRoot
	// and zero-length is deterministic. Just assert it doesn't change
	// across calls.
	r2, err := BytesRoot([]byte{})
	require.NoError(t, err)
	require.Equal(t, r, r2)
}
