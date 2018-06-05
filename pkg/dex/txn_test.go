package dex

import (
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/helinwang/dex/pkg/consensus"
	"github.com/stretchr/testify/assert"
)

func TestOrderEncodeDecode(t *testing.T) {
	o := Order{
		Owner:        consensus.Addr{1, 2, 3},
		SellSide:     true,
		QuantUnit:    1000000000,
		PriceUnit:    20000000,
		PlacedHeight: 1000,
		ExpireHeight: 1001,
	}
	b, err := rlp.EncodeToBytes(&o)
	if err != nil {
		panic(err)
	}

	var o1 Order
	err = rlp.DecodeBytes(b, &o1)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, o, o1)
}
