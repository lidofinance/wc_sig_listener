package entity

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	consensus "github.com/umbracle/go-eth-consensus"
)

var DOMAIN_BLS_TO_EXECUTION_CHANGE = [4]byte{10, 0, 0, 0}

//go:generate ../bin/sszgen -path=../entity/bls_to_execution_change.go -objs=BLSToExecutionChange -output ../entity/bls_to_execution_change_encoded.go
type BLSToExecutionChange struct {
	ValidatorIndex     uint64   `json:"validator_index" ssz-size:"48"`
	Pubkey             [48]byte `json:"from_bls_pubkey" ssz-size:"48"`
	ToExecutionAddress [32]byte `json:"to_execution_address" ssz-size:"32"`
}

func (b *BLSToExecutionChange) String() string {
	type humanReadable struct {
		ValidatorIndex     uint64 `json:"validator_index"`
		Pubkey             string `json:"from_bls_pubkey"`
		ToExecutionAddress string `json:"to_execution_address"`
	}

	obj := &humanReadable{
		ValidatorIndex:     b.ValidatorIndex,
		Pubkey:             hex.EncodeToString(b.Pubkey[:]),
		ToExecutionAddress: hex.EncodeToString(b.ToExecutionAddress[:]),
	}

	bb, _ := json.Marshal(obj)
	var prettyJSON bytes.Buffer
	_ = json.Indent(&prettyJSON, bb, "", "\t")

	return string(prettyJSON.Bytes())
}

func (b *BLSToExecutionChange) GetRoot(forkVersion [4]byte) ([32]byte, error) {
	sr, err := b.HashTreeRoot()
	if err != nil {
		return [32]byte{}, err
	}

	domain, err := consensus.ComputeDomain(DOMAIN_BLS_TO_EXECUTION_CHANGE, forkVersion, [32]byte{})
	if err != nil {
		return [32]byte{}, err
	}

	root, err := (&consensus.SigningData{ObjectRoot: sr, Domain: domain}).HashTreeRoot()
	if err != nil {
		return [32]byte{}, err
	}

	return root, nil
}
