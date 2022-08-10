package proxy

import (
	"encoding/hex"
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/sencha-dev/powkit/ethash"

	"github.com/sammy007/open-ethereum-pool/util"
)

var client = ethash.NewEthereum()

func (s *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, params []string, diff float64) (bool, bool) {
	hashNoNonce := params[1]
	mixDiges := strings.Replace(params[2], "0x", "", -1)
	shareDiff := int64(diff * float64(util.Pow2x32))

	h, ok := t.headers[hashNoNonce]
	if !ok {
		log.Printf("Stale share from %v@%v", login, ip)
		return false, false
	}

	hash := util.MustDecodeHex(params[1])
	height := uint64(h.height)
	nonce, _ := strconv.ParseUint(strings.Replace(params[0], "0x", "", -1), 16, 64)

	mix, digest, err := client.Compute(hash, height, nonce)
	if err != nil {
		return false, false
	}

	if hex.EncodeToString(mix) != mixDiges {
		return false, false
	}

	shareTarget := new(big.Int).SetBytes(digest)

	jobTargetBytes := util.MakeTarget(s.config.Proxy.Difficulty).Bytes()
	jobTarget := new(big.Int).SetBytes(jobTargetBytes)

	isValidShare := jobTarget.Cmp(shareTarget) >= 0
	isValidBlock := h.target.Cmp(shareTarget) >= 0

	if !isValidShare {
		return false, false
	}

	if s.config.Proxy.Stratum.Debug {
		actualShare, _ := new(big.Rat).SetFrac(util.Diff1, shareTarget).Float64()
		networkDiff := h.diff.Int64() / util.Pow2x32
		log.Printf("Valid share at height %d %.4f/%.4f from %s:%s@%s", h.height, actualShare, networkDiff, login, id, ip)
	}

	if isValidBlock {
		ok, err := s.rpc().SubmitBlock(params)
		if err != nil {
			log.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, params, shareDiff, h.diff.Int64(), h.height, s.hashrateExpiration)
			if exist {
				return true, false
			}
			if err != nil {
				log.Println("Failed to insert block candidate into backend:", err)
			} else {
				log.Printf("Inserted block %v to backend", h.height)
			}
			log.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := s.backend.WriteShare(login, id, params, shareDiff, h.height, s.hashrateExpiration)
		if exist {
			return true, false
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, true
}
