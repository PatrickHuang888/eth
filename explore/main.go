package main

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/params"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type LocalNode struct {
	key *ecdsa.PrivateKey
}

func init()  {
	log.SetLevel(log.TraceLevel)
}

func main() {
	var sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigs)


	key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}

	//ln := &LocalNode{key: key}

	bn:= enode.MustParse(params.GoerliBootnodes[0])


	from:= v4wire.Endpoint{IP: net.IP(enr.IPv4{127, 0, 0, 1}), UDP: 30303}
	to := v4wire.Endpoint{IP: bn.IP(), UDP: uint16(bn.UDP())}
	expiration:= 5*time.Second

	ping := &v4wire.Ping{
		Version:    4,
		From:       from,
		To:         to,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	packet, _, err :=v4wire.Encode(key, ping)
	if err!=nil {
		panic(err)
	}


	udpConn, err :=net.DialUDP("udp", nil, &net.UDPAddr{IP: bn.IP(), Port: bn.UDP()})
	if err!=nil {
		panic(err)
	}

	log.Traceln("send ping")
	_, err = udpConn.Write(packet)
	if err!=nil {
		panic(err)
	}

	var buf [512]byte
	log.Traceln("read")
	n, err :=udpConn.Read(buf[:])
	if err!=nil {
		log.Error(err)
	}
	//v4wire.Decode(buf[:n])
	log.Infof("read %d", n)

	udpConn.Close()

	<- sigs
}
