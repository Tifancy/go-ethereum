// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package les

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethapi"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/light"
	rpc "github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
)

type LesApiBackend struct {
	eth      *LightNodeService
	SolcPath string
	solc     *compiler.Solidity
	gpo      *gasprice.LightPriceOracle
}

func (b *LesApiBackend) SetHead(number uint64) {
	b.eth.blockchain.SetHead(number)
}

func (b *LesApiBackend) HeaderByNumber(blockNr rpc.BlockNumber) *types.Header {
	if blockNr == rpc.LatestBlockNumber || blockNr == rpc.PendingBlockNumber {
		return b.eth.blockchain.CurrentHeader()
	}

	return b.eth.blockchain.GetHeaderByNumber(uint64(blockNr))
}

func (b *LesApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	header := b.HeaderByNumber(blockNr)
	if header == nil {
		return nil, nil
	}
	return b.GetBlock(ctx, header.Hash())
}

func (b *LesApiBackend) StateByNumber(blockNr rpc.BlockNumber) (ethapi.State, error) {
	header := b.HeaderByNumber(blockNr)
	if header == nil {
		return nil, nil
	}
	return light.NewLightState(light.StateTrieID(header), b.eth.odr), nil
}

func (b *LesApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return b.eth.blockchain.GetBlock(ctx, blockHash)
}

func (b *LesApiBackend) GetState(header *types.Header) (ethapi.State, error) {
	id := &light.TrieID{
		BlockHash: header.Hash(),
		Root:      header.Root,
	}
	return light.NewLightState(id, b.eth.odr), nil
}

func (b *LesApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return light.GetBlockReceipts(ctx, b.eth.odr, blockHash)
}

func (b *LesApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.eth.blockchain.GetTd(blockHash)
}

func (b *LesApiBackend) GetVMEnv(ctx context.Context, msg core.Message, header *types.Header) (vm.Environment, func() error, error) {
	stateDb := light.NewLightState(light.StateTrieID(header), b.eth.odr)
	stateDb = stateDb.Copy()
	addr, _ := msg.From()
	from, err := stateDb.GetOrNewStateObject(ctx, addr)
	if err != nil {
		return nil, nil, err
	}
	from.SetBalance(common.MaxBig)
	env := light.NewEnv(ctx, stateDb, b.eth.chainConfig, b.eth.blockchain, msg, header, b.eth.chainConfig.VmConfig)
	return env, env.Error, nil
}

func (b *LesApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.eth.txPool.Add(ctx, signedTx)
}

func (b *LesApiBackend) RemoveTx(txHash common.Hash) {
	b.eth.txPool.RemoveTx(txHash)
}

func (b *LesApiBackend) GetPoolTransactions() types.Transactions {
	return b.eth.txPool.GetTransactions()
}

func (b *LesApiBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	return b.eth.txPool.GetTransaction(txHash)
}

func (b *LesApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.eth.txPool.GetNonce(ctx, addr)
}

func (b *LesApiBackend) Stats() (pending int, queued int) {
	return b.eth.txPool.Stats(), 0
}

func (b *LesApiBackend) TxPoolContent() (map[common.Address]map[uint64][]*types.Transaction, map[common.Address]map[uint64][]*types.Transaction) {
	return make(map[common.Address]map[uint64][]*types.Transaction), make(map[common.Address]map[uint64][]*types.Transaction)
}

func (b *LesApiBackend) Solc() (*compiler.Solidity, error) {
	var err error
	if b.solc == nil {
		b.solc, err = compiler.New(b.SolcPath)
	}
	return b.solc, err
}

func (b *LesApiBackend) SetSolc(solcPath string) (*compiler.Solidity, error) {
	b.SolcPath = solcPath
	b.solc = nil
	return b.Solc()
}

func (b *LesApiBackend) Downloader() *downloader.Downloader {
	return b.eth.Downloader()
}

func (b *LesApiBackend) ProtocolVersion() int {
	return b.eth.LesVersion() + 10000
}

func (b *LesApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *LesApiBackend) ChainDb() ethdb.Database {
	return b.eth.chainDb
}

func (b *LesApiBackend) EventMux() *event.TypeMux {
	return b.eth.eventMux
}

func (b *LesApiBackend) AccountManager() *accounts.Manager {
	return b.eth.accountManager
}
