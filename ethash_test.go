// Copyright 2015 The go-ethereum Authors
// Copyright 2015 Lefteris Karapetsas <lefteris@refu.co>
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethash

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func init() {
	// glog.SetV(6)
	// glog.SetToStderr(true)
}

type testBlock struct {
	difficulty  *big.Int
	hashNoNonce common.Hash
	nonce       uint64
	mixDigest   common.Hash
	number      uint64
}

func (b *testBlock) Difficulty() *big.Int     { return b.difficulty }
func (b *testBlock) HashNoNonce() common.Hash { return b.hashNoNonce }
func (b *testBlock) Nonce() uint64            { return b.nonce }
func (b *testBlock) MixDigest() common.Hash   { return b.mixDigest }
func (b *testBlock) NumberU64() uint64        { return b.number }

var validBlocks = []*testBlock{
	//frontierBlock1 = TestBlock
	//                   (BS8.pack "85913a3057ea8bec78cd916871ca73802e77724e014dda65add3405d02240eb7")
	//                   (BS8.pack "969b900de27b6ac6a67742365dd65f55a0526c41fd18e1b16f1a1215c2e66f59")
	//                   (integer2ByteString 6024642674226569000)
	//                   17171480576 --539bd4979fef1ec4
	//																					                    1
	// fake block 666
	//{
	//	number:      666,
	//	hashNoNonce: common.HexToHash("0000000000000000000000000000000000000000000000000000000000000001"),
	//	difficulty:  big.NewInt(1),
	//	nonce:       0x0000000000000001,
	//	mixDigest:   common.HexToHash("0000000000000000000000000000000000000000000000000000000000000001"),
	//},
	// frontier block 0
	//{
	//	number:      0,
	//	hashNoNonce: common.HexToHash("85913a3057ea8bec78cd916871ca73802e77724e014dda65add3405d02240eb7"),
	//	difficulty:  big.NewInt(17171480576),
	//	nonce:       0x539bd4979fef1ec4,
	//	mixDigest:   common.HexToHash("969b900de27b6ac6a67742365dd65f55a0526c41fd18e1b16f1a1215c2e66f59"),
	//},
	// from parity ethash test
	{
		number:      1,
		hashNoNonce: common.HexToHash("f57e6f3acfc0dd4b5bf2bee40ab3358aa68773a8d09f5e595eab559405527d72"),
		difficulty:  big.NewInt(9166922271705),
		nonce:       0xd7b3ac70a301a249,
		mixDigest:   common.HexToHash("1fff04cec94173fd591e3d8960ce6bdf8b1971048c71ff937bb2d32a6431ab6d"),
	},
	// from proof of concept nine testnet, epoch 0
	{
		number:      22,
		hashNoNonce: common.HexToHash("372eca2454ead349c3df0ab5d00b0b706b23e49d469387db91811cee0358fc6d"),
		difficulty:  big.NewInt(132416),
		nonce:       0x495732e0ed7a801c,
		mixDigest:   common.HexToHash("2f74cdeb198af0b9abe65d22d372e22fb2d474371774a9583c1cc427a07939f5"),
	},
	// from proof of concept nine testnet, epoch 1
	//{
	//	number:      30001,
	//	hashNoNonce: common.HexToHash("7e44356ee3441623bc72a683fd3708fdf75e971bbe294f33e539eedad4b92b34"),
	//	difficulty:  big.NewInt(1532671),
	//	nonce:       0x318df1c8adef7e5e,
	//	mixDigest:   common.HexToHash("144b180aad09ae3c81fb07be92c8e6351b5646dda80e6844ae1b697e55ddde84"),
	//},
	// from proof of concept nine testnet, epoch 2
	//{
	//	number:      60000,
	//	hashNoNonce: common.HexToHash("5fc898f16035bf5ac9c6d9077ae1e3d5fc1ecc3c9fd5bee8bb00e810fdacbaa0"),
	//	difficulty:  big.NewInt(2467358),
	//	nonce:       0x50377003e5d830ca,
	//	mixDigest:   common.HexToHash("ab546a5b73c452ae86dadd36f0ed83a6745226717d3798832d1b20b489e82063"),
	//},
}

var invalidZeroDiffBlock = testBlock{
	number:      61440000,
	hashNoNonce: crypto.Sha3Hash([]byte("foo")),
	difficulty:  big.NewInt(0),
	nonce:       0xcafebabec00000fe,
	mixDigest:   crypto.Sha3Hash([]byte("bar")),
}

func TestEthashVerifyValid(t *testing.T) {
	eth := New()
	for i, block := range validBlocks {
		fmt.Println("\n-----------------------------\nHello, 块", i, block.number)
		if !eth.Verify(block) {
			t.Errorf("block %d (%x) did not validate.", i, block.hashNoNonce[:6])
		}
	}
}

func rTestEthashVerifyInvalid(t *testing.T) {
	eth, err := NewForTesting()
	if err != nil {
		t.Fatal(err)
	}
	if eth.Verify(&invalidZeroDiffBlock) {
		t.Errorf("should not validate - we just ensure it does not panic on this block")
	}
}

func rTestEthashConcurrentVerify(t *testing.T) {
	eth, err := NewForTesting()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(eth.Full.Dir)

	block := &testBlock{difficulty: big.NewInt(10)}
	nonce, md := eth.Search(block, nil, 0)
	block.nonce = nonce
	block.mixDigest = common.BytesToHash(md)

	// Verify the block concurrently to check for data races.
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			if !eth.Verify(block) {
				t.Error("Block could not be verified")
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func rTestEthashConcurrentSearch(t *testing.T) {
	eth, err := NewForTesting()
	if err != nil {
		t.Fatal(err)
	}
	eth.Turbo(true)
	defer os.RemoveAll(eth.Full.Dir)

	type searchRes struct {
		n  uint64
		md []byte
	}

	var (
		block   = &testBlock{difficulty: big.NewInt(35000)}
		nsearch = 10
		wg      = new(sync.WaitGroup)
		found   = make(chan searchRes)
		stop    = make(chan struct{})
	)
	rand.Read(block.hashNoNonce[:])
	wg.Add(nsearch)
	// launch n searches concurrently.
	for i := 0; i < nsearch; i++ {
		go func() {
			nonce, md := eth.Search(block, stop, 0)
			select {
			case found <- searchRes{n: nonce, md: md}:
			case <-stop:
			}
			wg.Done()
		}()
	}

	// wait for one of them to find the nonce
	res := <-found
	// stop the others
	close(stop)
	wg.Wait()

	block.nonce = res.n
	block.mixDigest = common.BytesToHash(res.md)
	if !eth.Verify(block) {
		t.Error("Block could not be verified")
	}
}

func rTestEthashSearchAcrossEpoch(t *testing.T) {
	eth, err := NewForTesting()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(eth.Full.Dir)

	for i := epochLength - 40; i < epochLength+40; i++ {
		block := &testBlock{number: i, difficulty: big.NewInt(90)}
		rand.Read(block.hashNoNonce[:])
		nonce, md := eth.Search(block, nil, 0)
		block.nonce = nonce
		block.mixDigest = common.BytesToHash(md)
		if !eth.Verify(block) {
			t.Fatalf("Block could not be verified")
		}
	}
}

func rTestGetSeedHash(t *testing.T) {
	seed0, err := GetSeedHash(0)
	if err != nil {
		t.Errorf("Failed to get seedHash for block 0: %v", err)
	}
	if bytes.Compare(seed0, make([]byte, 32)) != 0 {
		log.Printf("seedHash for block 0 should be 0s, was: %v\n", seed0)
	}
	seed1, err := GetSeedHash(30000)
	if err != nil {
		t.Error(err)
	}

	// From python:
	// > from pyethash import get_seedhash
	// > get_seedhash(30000)
	expectedSeed1, err := hex.DecodeString("290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563")
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(seed1, expectedSeed1) != 0 {
		log.Printf("seedHash for block 1 should be: %v,\nactual value: %v\n", expectedSeed1, seed1)
	}

}
