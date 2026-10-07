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

// Package forkchoice implements the Iguazu-specific fork-choice scoring
// logic.  It is a Prysm-free port of the coordinator's
// beacon-chain/forkchoice/protoarray/consensus.go.
//
// The heavy LMD-GHOST computation (Head, insertNode, vote accounting) stays in
// the coordinator and is accessed through the iface.ForkChoice interface.
// Only the T(G)-tree filtering that classifies nodes as "acceptable" according
// to the optimistic spine set is ported here.
package forkchoice
