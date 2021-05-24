package main

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type LocalNode struct {
	key *ecdsa.PrivateKey
}

func init() {
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

	ctx, cancel := context.WithCancel(context.Background())

	//ln := &LocalNode{key: key}

	//bn:= enode.MustParse(params.GoerliBootnodes[0])
	bn := enode.MustParseV4(params.MainnetBootnodes[0])


	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()

		log.Infof("dial %s", bn.IP())
		udpConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: bn.IP(), Port: bn.UDP()})
		if err != nil {
			log.Error(err)
			return
		}
		defer udpConn.Close()

		from := v4wire.Endpoint{IP: net.IP(enr.IPv4{127, 0, 0, 1}), UDP: 30303}
		to := v4wire.Endpoint{IP: bn.IP(), UDP: uint16(bn.UDP())}
		expiration := 5 * time.Second

		ping := &v4wire.Ping{
			Version:    4,
			From:       from,
			To:         to,
			Expiration: uint64(time.Now().Add(expiration).Unix()),
		}
		packet, _, err := v4wire.Encode(key, ping)
		if err != nil {
			log.Error(err)
			return
		}
		log.Traceln("send ping")
		_, err = udpConn.Write(packet)
		if err != nil {
			log.Error(err)
			return
		}

		var buf [1024]byte
		/*buf := bytes.Buffer{}
		_, err = buf.ReadFrom(udpConn)*/
		n, err :=udpConn.Read(buf[:])
		if err != nil {
			log.Error(err)
			return
		}
		rspPacket, _, _, err:= v4wire.Decode(buf[:n])
		if err!=nil {
			log.Error(err)
			return
		}
		pong, ok := rspPacket.(*v4wire.Pong)
		if !ok {
			log.Error("response is not Pong")
		}
		log.Infof("received pong to %s", pong.To.IP.String())


		findNode := &v4wire.Findnode{Target: v4wire.EncodePubkey(&key.PublicKey),
			Expiration: uint64(time.Now().Add(expiration).Unix())}
		packet, _, err = v4wire.Encode(key, findNode)
		if err !=nil {
			log.Error(err)
			return
		}
		log.Infof("send find nodes")
		_, err = udpConn.Write(packet)
		if err != nil {
			log.Error(err)
			return
		}

		n, err =udpConn.Read(buf[:])
		if err!=nil {
			log.Error(errors.WithStack(err))
			return
		}
		rspPacket, _, _, err= v4wire.Decode(buf[:n])
		if err!=nil {
			log.Error(errors.WithStack(err))
			return
		}
		if rspPacket.Kind()!=v4wire.NeighborsPacket {
			log.Error("response not neighbors")
			return
		}
		neighbors := rspPacket.(*v4wire.Neighbors)
		for _, v := range neighbors.Nodes {
			log.Infof("got neighbor %s"+ v.IP.String())
		}

		/*switch rspPacket.Kind() {
		case v4wire.PingPacket:
			ping = rspPacket.(*v4wire.Ping)

		}*/

	}()


	wg.Wait()
	log.Infof("main wait ...")
	select {
	case <-sigs:
		cancel()
	case <-ctx.Done():
	}

}