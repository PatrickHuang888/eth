package main

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
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

		if err := lo.do(); err != nil {
			log.Errorf("%+v", err)
		}
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

	conns map[*node]*net.UDPConn
}

func (lo *local) init() error {
	key, err := crypto.GenerateKey()
	if err != nil {
		return errors.WithStack(err)
	}
	lo.key = key
	lo.expiration = 20 * time.Second
	lo.ep = v4wire.Endpoint{}

	lo.conns = make(map[*node]*net.UDPConn)

	/*db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, key)
	log.Info(ln.Node().IP().String())
	nd.localNode= ln*/
	return nil
}

func (lo *local) sendPing(nd *node) error {
	c := lo.conns[nd]

	to := v4wire.Endpoint{IP: nd.IP, UDP: uint16(nd.UDP)}
	ping := &v4wire.Ping{
		Version:    4,
		From:       lo.ep,
		To:         to,
		Expiration: uint64(time.Now().Add(lo.expiration).Unix()),
	}
	if err := writePacket(c, lo.key, ping); err != nil {
		return err
	}
	return nil
}

func (lo *local) readPong(nd *node) (*v4wire.Pong, error) {
	c := lo.conns[nd]

	packet, _, _, err := readPacket(c)
	if packet.Kind() != v4wire.PongPacket {
		return nil, errors.Errorf("%s not pong", packet.Name())
	}
	return packet.(*v4wire.Pong), err
}

func (lo *local) readPing(nd *node) (*v4wire.Ping, []byte, error) {
	c := lo.conns[nd]

	packet, _, hash, err := readPacket(c)
	if packet.Kind() != v4wire.PingPacket {
		return nil, nil, errors.Errorf("%s not pong", packet.Name())
	}
	return packet.(*v4wire.Ping), hash, err
}

func (lo *local) sendPong(nd *node, hash []byte) error {
	c := lo.conns[nd]

	to := v4wire.Endpoint{IP: nd.IP, UDP: uint16(nd.UDP)}
	pong := &v4wire.Pong{
		To:         to,
		ReplyTok:   hash,
		Expiration: uint64(time.Now().Add(lo.expiration).Unix()),
	}
	if err := writePacket(c, lo.key, pong); err != nil {
		return err
	}
	return nil
}

func (lo *local) sendFindNodes(nd *node) error {
	c := lo.conns[nd]

	findNode := &v4wire.Findnode{Target: v4wire.EncodePubkey(&lo.key.PublicKey),
		Expiration: uint64(time.Now().Add(lo.expiration).Unix()),
	}
	if err := writePacket(c, lo.key, findNode); err != nil {
		return err
	}
	return nil
}

func (lo *local) readNeighbors(nd *node) ([]v4wire.Node, error) {
	c := lo.conns[nd]

	packet, _, _, err := readPacket(c)
	if packet.Kind() != v4wire.NeighborsPacket {
		return nil, errors.Errorf("%s not neighbors", packet.Name())
	}
	neighbors := packet.(*v4wire.Neighbors)
	return neighbors.Nodes, err
}

func (lo *local) do() error {

	bn := enode.MustParseV4(params.MainnetBootnodes[0])
	boot := &node{IP: bn.IP(), UDP: uint16(bn.UDP()), key: bn.Pubkey()}

	log.Infof("connect boot %s", boot.String())
	if err := lo.connect(boot); err != nil {
		return err
	}
	defer lo.closeConnection(boot)

	log.Infof("send ping to boot")
	if err := lo.sendPing(boot); err != nil {
		return err
	}

	pong, err := lo.readPong(boot)
	if err != nil {
		return err
	}
	log.Infof("got pong from boot, pong to %s and udp %d", pong.To.IP.String(), pong.To.UDP)

	ping, hash, err := lo.readPing(boot)
	if err != nil {
		return err
	}
	log.Infof("got ping from %s to %s", ping.From.IP.String(), ping.To.IP.String())

	if err := lo.sendPong(boot, hash); err != nil {
		return err
	}

	log.Infof("send find nodes to boot")
	if err:=lo.sendFindNodes(boot);err!=nil {
		return err
	}

	nbs, err := lo.readNeighbors(boot)
	if err!=nil {
		return err
	}

	//lk := crypto.FromECDSAPub(&lo.key.PublicKey)[1:]  //v4
	localPubKey:= v4wire.EncodePubkey(&lo.key.PublicKey)
	localId := enode.ID(crypto.Keccak256Hash(localPubKey[:]))

	//log.Infof("local id: %s", hex.EncodeToString(localId[:2]))

	//bucketSize      := 16 // Kademlia bucket size
	//HashLength = 32
	hashBits          := len(common.Hash{}) * 8 // 32*8
	nBuckets          := hashBits / 15       // Number of buckets
	bucketMinDistance := hashBits - nBuckets // Log distance of closest bucket, 239

	for _, v := range nbs {
		k, err := v4wire.DecodePubkey(crypto.S256(), v.ID)
		if err!=nil {
			return err
		}
		nd := &node{IP:v.IP, UDP: v.UDP, key: k}
		log.Infof("got neighbor %s", nd.String())
		id := enode.ID(crypto.Keccak256Hash(v.ID[:]))
		//log.Infof("id: %s", hex.EncodeToString(id[:]))
		d:= enode.LogDist(localId, id)
		log.Infof("distance %d", d-bucketMinDistance-1)
	}


	log.Traceln("do exit")
	return nil
}

type node struct {
	key *ecdsa.PublicKey

	IP  net.IP
	UDP uint16
}

func (nd node) String() string{
	return fmt.Sprintf("ip %s, udp %d", nd.IP.String(), nd.UDP)
}

func (lo *local) closeConnection(nd *node) {
	lo.conns[nd].Close()
}

func (lo *local) connect(nd *node) error {
	log.Debugf("dial %s", nd.IP)
	c, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: nd.IP, Port: int(nd.UDP)})
	if err != nil {
		return errors.WithStack(err)
	}
	lo.conns[nd] = c
	return err
}

func readPacket(conn *net.UDPConn) (packet v4wire.Packet, fromKey v4wire.Pubkey, hash []byte, err error) {
	var buf [1280]byte // maxPacketSize = 1280
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
	log.Tracef("write packet %s", packet.Name())
	n, err := conn.Write(p)
	if err != nil {
		return errors.WithStack(err)
	}
	if n != len(p) {
		return errors.New("not wrote full packet")
	}
	return nil
}

/*func (nd *node) listenUdp() error {
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
*/
