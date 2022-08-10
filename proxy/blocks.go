package proxy

import (
	"encoding/hex"
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/sammy007/open-ethereum-pool/util"
)

const maxBacklog = 3

type heightDiffPair struct {
	diff   *big.Int
	height uint64
	target *big.Int
}

type BlockTemplate struct {
	sync.RWMutex
	Header     string
	Seed       string
	Target     string
	Difficulty *big.Int
	Height     uint64
	HeightHex  string
	headers    map[string]heightDiffPair
}

type Block struct {
	difficulty  *big.Int
	hashNoNonce common.Hash
	nonce       uint64
	mixDigest   common.Hash
	number      uint64
}

func (b Block) Difficulty() *big.Int     { return b.difficulty }
func (b Block) HashNoNonce() common.Hash { return b.hashNoNonce }
func (b Block) Nonce() uint64            { return b.nonce }
func (b Block) MixDigest() common.Hash   { return b.mixDigest }
func (b Block) NumberU64() uint64        { return b.number }

func (s *ProxyServer) fetchBlockTemplate() {
	r := s.rpc()
	t := s.currentBlockTemplate()
	reply, err := r.GetWork()
	if err != nil {
		log.Printf("Error while refreshing block template on %s: %s", r.Name, err)
		return
	}
	// No need to update, we have fresh job
	if t != nil && t.Header == reply[0] {
		return
	}
	diff := util.TargetHexToDiff(reply[2])
	height, err := strconv.ParseUint(strings.Replace(reply[3], "0x", "", -1), 16, 64)
	if err != nil {
		log.Printf("Error while Parse Uint block height")
	}

	blockTargetBytes, _ := hex.DecodeString(reply[2])
	blockTarget := new(big.Int).SetBytes(blockTargetBytes)

	newTemplate := BlockTemplate{
		Header:     reply[0],
		Seed:       reply[1],
		Target:     reply[2],
		Height:     height,
		HeightHex:  reply[3],
		Difficulty: diff,
		headers:    make(map[string]heightDiffPair),
	}
	// Copy job backlog and add current one
	newTemplate.headers[reply[0]] = heightDiffPair{
		diff:   diff,
		height: height,
		target: blockTarget,
	}
	if t != nil {
		for k, v := range t.headers {
			if v.height > height-maxBacklog {
				newTemplate.headers[k] = v
			}
		}
	}
	s.blockTemplate.Store(&newTemplate)
	log.Printf("New block to mine on %s at height %d / %s / %d", r.Name, height, reply[0][0:10], diff)

	// Stratum
	if s.config.Proxy.Stratum.Enabled {
		go s.broadcastNewJobs()
	}
}
