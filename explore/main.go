package main

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"net"
	"time"
)

type LocalNode struct {
	key *ecdsa.PrivateKey
}

func main() {
	/*key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}

	ln := &LocalNode{key: key}*/

	bn:= enode.MustParse(params.GoerliBootnodes[0])

}

func makePing(toaddr *net.UDPAddr) *v4wire.Ping {
	seq, _ := rlp.EncodeToBytes(t.localNode.Node().Seq())
	return &v4wire.Ping{
		Version:    4,
		From:       t.ourEndpoint(),
		To:         v4wire.NewEndpoint(toaddr, 0),
		Expiration: uint64(time.Now().Add(expiration).Unix()),
		Rest:       []rlp.RawValue{seq},
	}
}
