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

package forkchoice_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Iguazu/iguazu-consensus/forkchoice"
	"github.com/LFDT-Iguazu/iguazu-consensus/helpers"
	"github.com/LFDT-Iguazu/iguazu-types/common"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/iface"
	"github.com/LFDT-Iguazu/iguazu-types/coordinator/types"
)

// ---- mock iface.ForkChoiceNode ----------------------------------------------

type testNode struct {
	root         [32]byte
	slot         types.Slot
	cpFinalized  common.HashArray
	finalization common.HashArray
	prefix       common.HashArray
	published    common.HashArray
}

func (n *testNode) Root() [32]byte                       { return n.root }
func (n *testNode) Slot() types.Slot                     { return n.slot }
func (n *testNode) SpinesFinalized() common.HashArray    { return n.cpFinalized }
func (n *testNode) SpinesFinalization() common.HashArray { return n.finalization }
func (n *testNode) SpinesPrefix() common.HashArray       { return n.prefix }
func (n *testNode) SpinesPublished() common.HashArray    { return n.published }

// ---- mock iface.ForkChoiceFork ----------------------------------------------

type testFork struct {
	roots    [][32]byte
	nodesMap map[[32]byte]*testNode
}

func (f *testFork) Roots() [][32]byte { return f.roots }
func (f *testFork) NodeByRoot(root [32]byte) iface.ForkChoiceNode {
	n, ok := f.nodesMap[root]
	if !ok {
		return nil
	}
	return n
}

// ---- mock iface.ForkChoice --------------------------------------------------

type testFC struct{ forks []iface.ForkChoiceFork }

func (fc *testFC) HasNode(root [32]byte) bool       { return false }
func (fc *testFC) GetForks() []iface.ForkChoiceFork { return fc.forks }
func (fc *testFC) HeadBySubset(_ context.Context, acceptable map[[32]byte]struct{}, _ [32]byte) ([32]byte, error) {
	// return the lexicographically largest root among acceptable roots (deterministic stub)
	var best [32]byte
	for r := range acceptable {
		if bytes.Compare(r[:], best[:]) > 0 {
			best = r
		}
	}
	return best, nil
}

// ---- helpers ----------------------------------------------------------------

// nrToHash converts a small integer to a deterministic [32]byte (mirrors
// coordinator test helper bytesutil.ToBytes32([]byte(fmt.Sprintf("%d", i)))).
func nrToHash(i int) [32]byte {
	s := fmt.Sprintf("%d", i)
	var h [32]byte
	copy(h[:], s)
	return h
}

func h(b ...byte) common.Hash { return common.BytesToHash(b) }

func defCfg() *helpers.Config {
	return &helpers.Config{
		FcTgTreeForkSlot: 0, // always active (default: 0 <= any slot)
	}
}

// buildLinearFork builds a single fork with nodes 0…n-1 connected linearly.
// fn is called for each index to supply SpinesData fields.
func buildLinearFork(n int, fn func(i int) (cpFin, fin, prefix, spines common.HashArray)) *testFork {
	frk := &testFork{
		roots:    make([][32]byte, n),
		nodesMap: make(map[[32]byte]*testNode, n),
	}
	// roots[0] = tip (highest slot), roots[n-1] = oldest ancestor
	for i := 0; i < n; i++ {
		cpFin, fin, prefix, spines := fn(i)
		root := nrToHash(i)
		frk.roots[n-1-i] = root // oldest first → reverse below after building
		frk.nodesMap[root] = &testNode{
			root:         root,
			slot:         types.Slot(i),
			cpFinalized:  cpFin,
			finalization: fin,
			prefix:       prefix,
			published:    spines,
		}
	}
	// reverse so roots[0]=leaf (slot n-1), roots[n-1]=genesis (slot 0)
	for l, r := 0, len(frk.roots)-1; l < r; l, r = l+1, r-1 {
		frk.roots[l], frk.roots[r] = frk.roots[r], frk.roots[l]
	}
	return frk
}

// ---- pure function tests ----------------------------------------------------

func TestIsSequenceMatchOptimisticSpines(t *testing.T) {
	a, b, c := h('a'), h('b'), h('c')
	x := h('x')

	optSpines := []common.HashArray{
		{a, x},
		{b},
		{c, x},
	}

	// exact match
	assert.True(t, forkchoice.IsSequenceMatchOptimisticSpines(common.HashArray{a, b, c}, optSpines))
	// shorter seq is ok
	assert.True(t, forkchoice.IsSequenceMatchOptimisticSpines(common.HashArray{a, b}, optSpines))
	// empty seq always matches
	assert.True(t, forkchoice.IsSequenceMatchOptimisticSpines(nil, optSpines))
	// seq longer than optSpines → false
	assert.False(t, forkchoice.IsSequenceMatchOptimisticSpines(common.HashArray{a, b, c, x}, optSpines))
	// element not in slot → false
	assert.False(t, forkchoice.IsSequenceMatchOptimisticSpines(common.HashArray{b, b}, optSpines))
}

func TestIsPrefixMatchOptimisticSpines(t *testing.T) {
	a, b, c := h('a'), h('b'), h('c')
	x := h('x')

	optSpines := []common.HashArray{
		{x},
		{a, x},
		{x},
		{b, x},
		{c},
	}

	// all prefix elements appear in optSpines in order → true
	assert.True(t, forkchoice.IsPrefixMatchOptimisticSpines(common.HashArray{a, b, c}, optSpines))
	// empty prefix → always true
	assert.True(t, forkchoice.IsPrefixMatchOptimisticSpines(nil, optSpines))
	// element missing → false
	assert.False(t, forkchoice.IsPrefixMatchOptimisticSpines(common.HashArray{a, h('z')}, optSpines))
	// prefix longer than optSpines → false
	assert.False(t, forkchoice.IsPrefixMatchOptimisticSpines(
		common.HashArray{a, b, c, h('d'), h('e'), h('f')},
		optSpines,
	))
}

func TestIndexOfOptimisticSpines(t *testing.T) {
	a, b, c := h('a'), h('b'), h('c')
	x := h('x')
	optSpines := []common.HashArray{{x, a}, {b}, {c, x}}

	assert.Equal(t, 0, forkchoice.IndexOfOptimisticSpines(a, optSpines))
	assert.Equal(t, 1, forkchoice.IndexOfOptimisticSpines(b, optSpines))
	assert.Equal(t, 2, forkchoice.IndexOfOptimisticSpines(c, optSpines))
	assert.Equal(t, -1, forkchoice.IndexOfOptimisticSpines(h('z'), optSpines))
}

// ---- CollectTgTreeNodes tests -----------------------------------------------

// TestCollectTgTreeNodes_PrefixExtension mirrors the coordinator test
// Test_collectTgTreeNodesByOptimisticSpines_prefix_extension.
func TestCollectTgTreeNodes_PrefixExtension(t *testing.T) {
	// Build a single linear fork with 10 nodes (0..9) whose spine data
	// progressively extends prefix and finalization.
	cpFin := common.HashArray{h('x', 'x', 'x')}

	spinesData := []struct{ spines, prefix, fin common.HashArray }{
		{common.HashArray{}, common.HashArray{}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '2')}, common.HashArray{}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '2'), h('a', '3')}, common.HashArray{}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '2'), h('a', '3'), h('a', '4')}, common.HashArray{h('a', '2')}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '2'), h('a', '3'), h('a', '4')}, common.HashArray{h('a', '2')}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '3'), h('a', '4'), h('a', '5')}, common.HashArray{h('a', '2'), h('a', '3')}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '4'), h('a', '5'), h('a', '6')}, common.HashArray{h('a', '2'), h('a', '3'), h('a', '4')}, common.HashArray{h('a', '1')}},
		{common.HashArray{h('a', '5'), h('a', '6'), h('a', '7')}, common.HashArray{h('a', '3'), h('a', '4'), h('a', '5')}, common.HashArray{h('a', '1'), h('a', '2')}},
		{common.HashArray{h('a', '6'), h('a', '7'), h('a', '8')}, common.HashArray{h('a', '4'), h('a', '5'), h('a', '6')}, common.HashArray{h('a', '1'), h('a', '2'), h('a', '3')}},
		{common.HashArray{h('a', '7'), h('a', '8'), h('a', '9'), h('a', '1', '0')}, common.HashArray{h('a', '4'), h('a', '5'), h('a', '6'), h('a', '7')}, common.HashArray{h('a', '1'), h('a', '2'), h('a', '3')}},
	}

	frk := buildLinearFork(10, func(i int) (common.HashArray, common.HashArray, common.HashArray, common.HashArray) {
		d := spinesData[i]
		return cpFin, d.fin, d.prefix, d.spines
	})

	zero := nrToHash(0)
	optSpines := []common.HashArray{
		{h('a', '1')},
		{zero, zero, h('a', '2'), zero},
		{h('a', '3'), zero, zero},
		{zero, zero, zero, zero, zero, h('a', '4')},
		{zero, h('a', '5'), zero, zero},
		{zero, zero, zero, h('a', '6'), zero, zero},
		{h('a', '7')},
		{h('a', '8'), zero},
		{zero, zero, h('a', '9')},
		{h('a', '1', '0')},
	}

	jCpRoot := nrToHash(0)
	fc := &testFC{forks: []iface.ForkChoiceFork{frk}}
	result := forkchoice.CollectTgTreeNodes(fc, optSpines, jCpRoot, defCfg())

	// All 10 nodes should be accepted
	require.Len(t, result, 10)
	for i := 0; i < 10; i++ {
		assert.Contains(t, result, nrToHash(i), "node %d should be in acceptable set", i)
	}
}

// TestCollectTgTreeNodes_NotJustified: a fork that doesn't pass through
// jCpRoot must be excluded entirely.
func TestCollectTgTreeNodes_NotJustified(t *testing.T) {
	cpFin := common.HashArray{h('x')}
	frk := buildLinearFork(3, func(i int) (common.HashArray, common.HashArray, common.HashArray, common.HashArray) {
		return cpFin, common.HashArray{h('a')}, common.HashArray{}, common.HashArray{}
	})

	jCpRoot := [32]byte{0xFF} // not in the fork
	fc := &testFC{forks: []iface.ForkChoiceFork{frk}}
	result := forkchoice.CollectTgTreeNodes(fc, []common.HashArray{{h('a')}}, jCpRoot, defCfg())
	assert.Empty(t, result)
}

// TestCollectTgTreeNodes_EmptyCpFinalized: nodes with empty cpFinalized are skipped.
func TestCollectTgTreeNodes_EmptyCpFinalized(t *testing.T) {
	frk := buildLinearFork(2, func(i int) (common.HashArray, common.HashArray, common.HashArray, common.HashArray) {
		return nil, common.HashArray{h('a')}, common.HashArray{}, common.HashArray{}
	})

	jCpRoot := nrToHash(0)
	fc := &testFC{forks: []iface.ForkChoiceFork{frk}}
	result := forkchoice.CollectTgTreeNodes(fc, []common.HashArray{{h('a')}}, jCpRoot, defCfg())
	assert.Empty(t, result)
}

// TestCollectTgTreeNodes_NoPublished: a node with no published spines
// (len(published)==0) is always accepted once finalization and prefix match.
func TestCollectTgTreeNodes_NoPublished(t *testing.T) {
	cpFin := common.HashArray{h('x')}
	a := h('a')
	frk := buildLinearFork(2, func(i int) (common.HashArray, common.HashArray, common.HashArray, common.HashArray) {
		return cpFin,
			common.HashArray{a}, // finalization
			common.HashArray{}, // prefix
			common.HashArray{} // no published spines
	})

	// optSpines[0] contains 'a' → finalization matches (IsFcTgTreeFork: existedSpines[a]=true)
	jCpRoot := nrToHash(1) // leaf = node 1 (slot 1)
	fc := &testFC{forks: []iface.ForkChoiceFork{frk}}
	result := forkchoice.CollectTgTreeNodes(fc, []common.HashArray{{a}}, jCpRoot, defCfg())
	// node at index i=1 (slot 0) should be accepted (no published → isExtended=true branch)
	assert.NotEmpty(t, result)
}

// TestGetParentByOptimisticSpines_NoAcceptable: empty optSpines →
// no acceptable nodes → returns zero hash without error.
func TestGetParentByOptimisticSpines_NoAcceptable(t *testing.T) {
	fc := &testFC{}
	root, err := forkchoice.GetParentByOptimisticSpines(context.Background(), fc, nil, [32]byte{}, defCfg())
	require.NoError(t, err)
	assert.Equal(t, [32]byte{}, root)
}
