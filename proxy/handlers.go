package proxy

import (
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/sammy007/open-ethereum-pool/util"
)

// Allow only lowercase hexadecimal with 0x prefix
var noncePattern = regexp.MustCompile("^0x[0-9a-f]{16}$")
var hashPattern = regexp.MustCompile("^0x[0-9a-f]{64}$")
var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_]{1,8}$")

// Stratum
func (s *ProxyServer) handleLoginRPC(cs *Session, params []string, id string) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	login := params[0]
	if strings.Contains(login, ".") {
		var workerParams = strings.Split(login, ".")
		login = workerParams[0]
		id = workerParams[1]
	}
	login = strings.ToLower(login)

	if !util.IsValidHexAddress(login) {
		return false, &ErrorReply{Code: -1, Message: "Invalid login"}
	}
	if !s.policy.ApplyLoginPolicy(login, cs.ip) {
		return false, &ErrorReply{Code: -1, Message: "You are blacklisted"}
	}
	if !workerPattern.MatchString(id) {
		id = "0"
	}
	cs.worker = id
	cs.login = login
	s.registerSession(cs)
	log.Printf("Stratum miner connected %v@%v", login, cs.ip)

	cs.diff = s.config.Proxy.Difficulty
	cs.nextDiff = cs.diff

	return true, nil
}


func (s *ProxyServer) handleGetWorkRPC(cs *Session) ([]string, *ErrorReply) {
	t := s.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || s.isSick() {
		return nil, &ErrorReply{Code: 0, Message: "Work not ready"}
	}

	return []string{t.Header, t.Seed, s.diff, t.HeightHex}, nil
}

// Stratum
func (s *ProxyServer) handleTCPSubmitRPC(cs *Session, id string, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return s.handleSubmitRPC(cs, cs.login, cs.worker, params)
}

func (s *ProxyServer) calcNextDiff(cs *Session) (newDiff float64) {
	ts := time.Now().Unix()
	options := s.config.Proxy.VarDiff
	difficulty := cs.diff

	if cs.LastRtc == 0 {
		cs.LastRtc = ts - (options.RetargetTime / 2)
		cs.LastTs = ts
		cs.TimeBuffer = NewRingBuffer(s.BufferSize)
		return
	}

	minDiff := options.MinDiff
	maxDiff := options.MaxDiff

	sinceLast := ts - cs.LastTs

	cs.TimeBuffer.Append(sinceLast)
	cs.LastTs = ts

	if (ts-cs.LastRtc) < options.RetargetTime && cs.TimeBuffer.Size() > 0 {
		return
	}

	cs.LastRtc = ts
	avg := cs.TimeBuffer.Avg()

	ddiff := float64(options.TargetTime) / avg

	if avg > s.tMax && difficulty > minDiff {
		if options.X2Mode {
			ddiff = 0.5
		}
		if (ddiff * difficulty) < minDiff {
			ddiff = minDiff / difficulty
		}
	} else if avg < s.tMin {
		if options.X2Mode {
			ddiff = 2
		}
		if (ddiff * difficulty) > maxDiff {
			ddiff = maxDiff / difficulty
		}
	} else {
		return difficulty
	}

	newDiff = difficulty * ddiff

	if newDiff < minDiff {
		newDiff = minDiff
	}

	if newDiff > maxDiff {
		newDiff = maxDiff
	}

	if newDiff < difficulty || newDiff > difficulty {
		cs.LastRtc = ts
		cs.PendingDiff = true
		cs.TimeBuffer.Clear()
		return
	}
	return newDiff
}

func (s *ProxyServer) handleSubmitRPC(cs *Session, login, id string, params []string) (bool, *ErrorReply) {
	if !workerPattern.MatchString(id) {
		id = "0"
	}
	if len(params) != 3 {
		s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed params from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	if !noncePattern.MatchString(params[0]) || !hashPattern.MatchString(params[1]) || !hashPattern.MatchString(params[2]) {
		s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed PoW result from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed PoW result"}
	}
	t := s.currentBlockTemplate()
	exist, validShare := s.processShare(login, id, cs.ip, t, params, cs.diff)
	ok := s.policy.ApplySharePolicy(cs.ip, !exist && validShare)

	if exist {
		log.Printf("Duplicate share from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: 22, Message: "Duplicate share"}
	}

	if !validShare {
		log.Printf("Invalid share from %s@%s", login, cs.ip)
		// Bad shares limit reached, return error and close
		if !ok {
			return false, &ErrorReply{Code: 23, Message: "Invalid share"}
		}
		return false, nil
	}

	if !ok {
		return true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
	}

	nextDiff := s.calcNextDiff(cs)
	if nextDiff != 0 {
		cs.nextDiff = nextDiff
	}

	return true, nil
}

func (s *ProxyServer) handleUnknownRPC(cs *Session, m string) *ErrorReply {
	log.Printf("Unknown request method %s from %s", m, cs.ip)
	s.policy.ApplyMalformedPolicy(cs.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}
