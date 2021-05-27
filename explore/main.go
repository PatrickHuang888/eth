package main

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"sync"
	"time"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

func main() {
	//var sigs = make(chan os.Signal, 1)
	//signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	//defer signal.Stop(sigs)

	lo := &local{}
	if err := lo.init(); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	/*if err := node.listenUdp(); err != nil {
		log.Errorf("%+v", err)
		os.Exit(1)
	}*/

	//ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	/*go func() {
		wg.Add(1)
		defer wg.Done()

		node.udpListenLoop()
	}()*/

	wg.Add(1)
	go func() {
		defer wg.Done()
		lo.do()
	}()

	wg.Wait()
	log.Traceln("main wait ...")
	/*select {
	case <-sigs:
		cancel()
	case <-ctx.Done():
	}*/
	log.Traceln("main exit")
}

type local struct {
	key        *ecdsa.PrivateKey
	expiration time.Duration

	ep v4wire.Endpoint
}

func (lo *local) init() error {
	key, err := crypto.GenerateKey()
	if err != nil {
		return errors.WithStack(err)
	}
	lo.key = key
	lo.expiration = 20 * time.Second
	lo.ep = v4wire.Endpoint{}

	/*db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, key)
	log.Info(ln.Node().IP().String())
	nd.localNode= ln*/
	return nil
}

func (lo *local) do() error {
	boot := &node{}
	bn := enode.MustParseV4(params.MainnetBootnodes[0])
	boot.nd = bn

	if err := boot.connect(); err != nil {
		return err
	}
	defer boot.close()

	//from := v4wire.Endpoint{IP: net.IP(enr.IPv4{58,250,158,172}), UDP: 44321}
	//from := v4wire.Endpoint{IP: net.IP(enr.IPv4{127, 0, 0, 1}), UDP: 30303}
	from := lo.ep
	to := v4wire.Endpoint{IP: bn.IP(), UDP: uint16(bn.UDP())}

	ping := &v4wire.Ping{
		Version:    4,
		From:       from,
		To:         to,
		Expiration: uint64(time.Now().Add(lo.expiration).Unix()),
	}
	packet, _, err := v4wire.Encode(lo.key, ping)
	if err != nil {
		log.Error(err)
		return
	}
	log.Traceln("send ping to boot node")
	_, err = udpConn.Write(packet)
	if err != nil {
		log.Error(errors.WithStack(err))
		return
	}

	//loop:
	//for {

	rsp, _, hash, err := nd.readPacket()
	if err != nil {
		log.Errorf("%+v", err)
		return
	}

	switch rsp.Kind() {
	case v4wire.PongPacket:
		pong := rsp.(*v4wire.Pong)
		log.Infof("received pong to %s udp %d tcp %d", pong.To.IP.String(), pong.To.UDP, pong.To.TCP)
		nd.local = pong.To

	default:
		log.Infof("read packet %d", rsp.Kind())
	}

	rsp, _, hash, err = nd.readPacket()
	if err != nil {
		log.Errorf("%+v", err)
		return
	}

	switch rsp.Kind() {
	case v4wire.PingPacket:
		ping := rsp.(*v4wire.Ping)
		log.Infof("received ping %s", ping.From.IP.String())
		pong := &v4wire.Pong{To: ping.From, ReplyTok: hash, Expiration: uint64(time.Now().Add(nd.expiration).Unix())}
		if err = nd.writePacket(pong); err != nil {
			log.Errorf("%+v", err)
			return
		}
	default:
		log.Infof("read packet %d", rsp.Kind())
	}

	findNode := &v4wire.Findnode{Target: v4wire.EncodePubkey(&nd.key.PublicKey), Expiration: uint64(time.Now().Add(nd.expiration).Unix())}
	packet, _, err = v4wire.Encode(nd.key, findNode)
	if err != nil {
		log.Error(errors.WithStack(err))
		return
	}
	log.Traceln("send findNode")
	_, err = udpConn.Write(packet)
	if err != nil {
		log.Error(errors.WithStack(err))
		return
	}
	rsp, _, hash, err = nd.readPacket()
	if err != nil {
		log.Errorf("%+v", err)
		return
	}
	switch rsp.Kind() {
	case v4wire.NeighborsPacket:
		log.Info("got neighbors packet")
		neighbors := rsp.(*v4wire.Neighbors)

		var wg sync.WaitGroup
		for _, n := range neighbors.Nodes {
			wg.Add(1)

			go func() {
				defer wg.Done()

				log.Infof("neighbor %s", n.IP)
				c, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: n.IP, Port: int(n.UDP)})
				if err != nil {
					log.Errorf("%+v", err)
					return
				}

				if err := c.sendPing(n); err != nil {
					log.Errorf("send ping error %+v", err)
					return
				}
			}()
		}
		wg.Wait()

	default:
		log.Infof("read packet %s", rsp.Name())
	}

	//}

	log.Traceln("do exit")
}

type node struct {
	nd *enode.Node
	//key        *ecdsa.PrivateKey

	conn *net.UDPConn

	//listenConn net.PacketConn
	//localNode *enode.LocalNode
}

func (nd *node) close() {
	nd.conn.Close()
}

func (nd *node) connect() error {
	log.Infof("dial %s", nd.nd.IP())
	var err error
	nd.conn, err = net.DialUDP("udp", nil, &net.UDPAddr{IP: nd.nd.IP(), Port: nd.nd.UDP()})
	if err != nil {
		return errors.WithStack(err)
	}
	return err
}

func readPacket(conn *net.UDPConn) (packet v4wire.Packet, fromKey v4wire.Pubkey, hash []byte, err error) {
	var buf [1280]byte // maxPacketSize = 1280
	/*buf := bytes.Buffer{}
	_, err = buf.ReadFrom(udpConn)*/
	n, err := conn.Read(buf[:])
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	packet, fromKey, hash, err = v4wire.Decode(buf[:n])
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

func writePacket(conn *net.UDPConn, key *ecdsa.PrivateKey, packet v4wire.Packet) error {
	p, _, err := v4wire.Encode(key, packet)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Debugf("write packet %s", packet.Name())
	if _, err := conn.Write(p); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (nd *node) read() error {
	var buf [1024]byte
	/*buf := bytes.Buffer{}
	_, err = buf.ReadFrom(udpConn)*/
	n, err := nd.udpConn.Read(buf[:])
	if err != nil {
		return errors.WithStack(err)
	}
	packet, _, hash, err := v4wire.Decode(buf[:n])
	if err != nil {
		return errors.WithStack(err)
	}

	switch packet.Kind() {
	case v4wire.PongPacket:
		pong := packet.(*v4wire.Pong)
		log.Infof("received pong to %s", pong.To.IP.String())
		nd.local = pong.To

	case v4wire.PingPacket:
		ping := packet.(*v4wire.Ping)
		log.Infof("received ping %s", ping.From.IP.String())
		pong := &v4wire.Pong{To: ping.From, ReplyTok: hash, Expiration: uint64(time.Now().Add(nd.expiration).Unix())}
		rspPacket, _, err := v4wire.Encode(nd.key, pong)
		if err != nil {
			return errors.WithStack(err)
		}
		log.Infof("write pong")
		if _, err := nd.udpConn.Write(rspPacket); err != nil {
			return errors.WithStack(err)
		}

	default:
		log.Infof("read packet %d", packet.Kind())
	}
	return nil
}

func (nd *node) listenUdp() error {
	addr := "0.0.0.0:0"
	listenConn, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return errors.WithStack(err)
	}
	nd.listenConn = listenConn
	return nil
}

func (nd *node) udpListenLoop() {
	log.Debugf("node udp listen loop")
	var buf [1280]byte // max 1280

	for {
		n, remoteAddr, err := nd.listenConn.ReadFrom(buf[:])
		if err != nil {
			log.Error(errors.WithStack(err))
			return
		}
		packet, _, hash, err := v4wire.Decode(buf[:n])
		if err != nil {
			log.Error(errors.WithStack(err))
			return
		}

		switch packet.Kind() {
		case v4wire.PingPacket:
			ping := packet.(*v4wire.Ping)
			log.Infof("receive ping from %s", remoteAddr.String())
			pong := &v4wire.Pong{To: ping.From, ReplyTok: hash, Expiration: uint64(time.Now().Add(nd.expiration).Unix())}
			packet, _, err := v4wire.Encode(nd.key, pong)
			if err != nil {
				log.Error(errors.WithStack(err))
				return
			}
			log.Traceln("send pong")
			if _, err := nd.listenConn.WriteTo(packet, remoteAddr); err != nil {
				log.Error(errors.WithStack(err))
				return
			}
		default:
			log.Infof("read packet %d", packet.Kind())
		}
	}
}
