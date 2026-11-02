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

package prevote

import (
	"context"
	"testing"
	"time"

	"github.com/prysmaticlabs/go-bitfield"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

func newTestPrevote(slot types.Slot, idx types.CommitteeIndex, bits bitfield.Bitlist) *types.PreVote {
	return &types.PreVote{
		AggregationBits: []byte(bits),
		Data: &types.PreVoteData{
			Slot:       slot,
			Index:      idx,
			Candidates: []byte{0x01, 0x02, 0x03},
		},
		Signature: make([]byte, 96),
	}
}

func TestPrevoteCache_SaveAndGetBySlot(t *testing.T) {
	c := NewPrevoteCache(time.Minute, 2*time.Minute)

	// Bitlist with committee size 8 — bit 2 set (i.e. validator index 2 voted).
	b := bitfield.NewBitlist(8)
	b.SetBitAt(2, true)

	pv := newTestPrevote(7, 0, b)
	require.NoError(t, c.SavePrevote(pv))

	got := c.GetPrevoteBySlot(context.Background(), 7)
	require.Len(t, got, 1)
	require.Equal(t, pv.Data.Slot, got[0].Data.Slot)
	require.Equal(t, pv.Data.Index, got[0].Data.Index)
	require.Equal(t, pv.AggregationBits, got[0].AggregationBits)

	// Different slot has no entries.
	require.Empty(t, c.GetPrevoteBySlot(context.Background(), 8))
}

func TestPrevoteCache_HasPrevoteCoversBits(t *testing.T) {
	c := NewPrevoteCache(time.Minute, 2*time.Minute)

	b := bitfield.NewBitlist(8)
	b.SetBitAt(1, true)
	b.SetBitAt(3, true)
	stored := newTestPrevote(3, 1, b)
	require.NoError(t, c.SavePrevote(stored))

	// Subset — covered.
	subset := bitfield.NewBitlist(8)
	subset.SetBitAt(1, true)
	has, err := c.HasPrevote(newTestPrevote(3, 1, subset))
	require.NoError(t, err)
	require.True(t, has)

	// Disjoint bit — not covered.
	disjoint := bitfield.NewBitlist(8)
	disjoint.SetBitAt(5, true)
	has, err = c.HasPrevote(newTestPrevote(3, 1, disjoint))
	require.NoError(t, err)
	require.False(t, has)

	// Different committee index — not covered.
	has, err = c.HasPrevote(newTestPrevote(3, 2, b))
	require.NoError(t, err)
	require.False(t, has)
}

func TestPrevoteCache_PurgeOutdated(t *testing.T) {
	c := NewPrevoteCache(time.Minute, 2*time.Minute)

	b := bitfield.NewBitlist(8)
	b.SetBitAt(0, true)

	require.NoError(t, c.SavePrevote(newTestPrevote(1, 0, b)))
	require.NoError(t, c.SavePrevote(newTestPrevote(2, 0, b)))
	require.NoError(t, c.SavePrevote(newTestPrevote(3, 0, b)))

	require.NoError(t, c.PurgeOutdatedPrevote(3))

	require.Empty(t, c.GetPrevoteBySlot(context.Background(), 1))
	require.Empty(t, c.GetPrevoteBySlot(context.Background(), 2))
	require.Len(t, c.GetPrevoteBySlot(context.Background(), 3), 1)

	// Purged bits are now in the seen-index: re-saving the same prevote at
	// slot 1 is a no-op.
	require.NoError(t, c.SavePrevote(newTestPrevote(1, 0, b)))
	require.Empty(t, c.GetPrevoteBySlot(context.Background(), 1))
}
