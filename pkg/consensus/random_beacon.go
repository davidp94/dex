package consensus

import (
	"errors"
	"fmt"
	"sync"
)

var errCommitteeNotSelected = errors.New("committee not selected yet")
var errAddrNotInCommittee = errors.New("addr not in committee")

// RandomBeacon is the round information.
//
// The random beacon, block proposal, block notarization advance to
// the next round in lockstep.
type RandomBeacon struct {
	cfg               Config
	mu                sync.Mutex
	nextRBCmteHistory []int
	nextNtCmteHistory []int
	nextBPCmteHistory []int
	groups            []*Group

	rbRand Rand
	ntRand Rand
	bpRand Rand

	curRoundShares map[Hash]*RandBeaconSigShare
	sigHistory     []*RandBeaconSig
}

// NewRandomBeacon creates a new random beacon
func NewRandomBeacon(seed Rand, groups []*Group, cfg Config) *RandomBeacon {
	rbRand := seed.Derive([]byte("random beacon committee rand seed"))
	bpRand := seed.Derive([]byte("block proposer committee rand seed"))
	ntRand := seed.Derive([]byte("notarization committee rand seed"))
	return &RandomBeacon{
		cfg:               cfg,
		groups:            groups,
		rbRand:            rbRand,
		bpRand:            bpRand,
		ntRand:            ntRand,
		nextRBCmteHistory: []int{rbRand.Mod(len(groups))},
		nextNtCmteHistory: []int{ntRand.Mod(len(groups))},
		nextBPCmteHistory: []int{bpRand.Mod(len(groups))},
		curRoundShares:    make(map[Hash]*RandBeaconSigShare),
		sigHistory: []*RandBeaconSig{
			{Sig: []byte("DEX random beacon 0th signature")},
		},
	}
}

// GetShare returns the randome beacon signature share of the current
// round.
func (r *RandomBeacon) GetShare(h Hash) *RandBeaconSigShare {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.curRoundShares[h]
}

// RecvRandBeaconSigShare receives one share of the random beacon
// signature.
func (r *RandomBeacon) RecvRandBeaconSigShare(s *RandBeaconSigShare, groupID int) (*RandBeaconSig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.round() != s.Round {
		return nil, fmt.Errorf("unexpected RandBeaconSigShare.Round: %d, expected: %d", s.Round, r.round())
	}

	if h := hash(r.sigHistory[s.Round-1].Sig); h != s.LastSigHash {
		return nil, fmt.Errorf("unexpected RandBeaconSigShare.LastSigHash: %x, expected: %x", s.LastSigHash, h)
	}

	r.curRoundShares[s.Hash()] = s
	fmt.Println(len(r.curRoundShares), r.cfg.GroupThreshold)
	if len(r.curRoundShares) >= r.cfg.GroupThreshold {
		sig := recoverRandBeaconSig(r.curRoundShares)
		var rbs RandBeaconSig
		rbs.LastRandVal = s.LastSigHash
		rbs.Round = s.Round
		msg := rbs.Encode(false)
		if !sig.Verify(&r.groups[groupID].PK, string(msg)) {
			panic("impossible: random beacon group signature verification failed")
		}

		rbs.Sig = sig.Serialize()
		return &rbs, nil
	}
	return nil, nil
}

// RecvRandBeaconSig adds the random beacon signature.
func (r *RandomBeacon) RecvRandBeaconSig(s *RandBeaconSig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.round() != s.Round {
		return fmt.Errorf("unexpected RandBeaconSig round: %d, expected: %d", s.Round, r.round())
	}

	r.deriveRand(hash(s.Sig))
	r.curRoundShares = make(map[Hash]*RandBeaconSigShare)
	r.sigHistory = append(r.sigHistory, s)
	return nil
}

func (r *RandomBeacon) round() int {
	return len(r.sigHistory)
}

// Round returns the round of the random beacon.
//
// This round will be always greater or equal to Chain.Round():
// - greater: when the node is synchronizing. It will synchronize the
// random beacon first, and then synchronize the chain's blocks.
// - equal: when the node is synchronized.
func (r *RandomBeacon) Round() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.round()
}

// Rank returns the rank for the given member in the current block
// proposal committee.
func (r *RandomBeacon) Rank(addr Addr) (int, error) {
	bp := r.nextBPCmteHistory[len(r.nextBPCmteHistory)-1]
	g := r.groups[bp]
	idx := -1
	for i := range g.Members {
		if addr == g.Members[i] {
			idx = i
			break
		}
	}

	if idx < 0 {
		return 0, errors.New("addr not in the current block proposal committee")
	}

	perm := r.bpRand.Perm(idx+1, len(g.Members))
	return perm[idx], nil
}

func (r *RandomBeacon) deriveRand(h Hash) {
	r.rbRand = r.rbRand.Derive(h[:])
	r.nextRBCmteHistory = append(r.nextRBCmteHistory, r.rbRand.Mod(len(r.groups)))
	r.ntRand = r.ntRand.Derive(h[:])
	r.nextNtCmteHistory = append(r.nextNtCmteHistory, r.ntRand.Mod(len(r.groups)))
	r.bpRand = r.bpRand.Derive(h[:])
	r.nextBPCmteHistory = append(r.nextBPCmteHistory, r.bpRand.Mod(len(r.groups)))
}

// Committees returns the current random beacon, block proposal,
// notarization committees.
func (r *RandomBeacon) Committees() (rb, bp, nt int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rb = r.nextRBCmteHistory[len(r.nextRBCmteHistory)-1]
	bp = r.nextBPCmteHistory[len(r.nextBPCmteHistory)-1]
	nt = r.nextNtCmteHistory[len(r.nextNtCmteHistory)-1]
	return
}

// History returns the random beacon signature history.
func (r *RandomBeacon) History() []*RandBeaconSig {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.sigHistory
}
