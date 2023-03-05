package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
)

type Node struct {
	Port int
}

func New(port int) *Node {
	return &Node{Port: port}
}

type discoveryNotifee struct {
	h host.Host
}

func (n *Node) Host() (host.Host, error) {
	addrs := []string{
		"/ip4/0.0.0.0/tcp/%d",
		"/ip4/0.0.0.0/udp/%d/quic",
		"/ip4/0.0.0.0/udp/%d/quic-v1",
		"/ip6/::/tcp/%d",
		"/ip6/::/udp/%d/quic",
		"/ip6/::/udp/%d/quic-v1",
	}
	listenAddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, s := range addrs {
		addr, addrErr := multiaddr.NewMultiaddr(fmt.Sprintf(s, n.Port))
		if addrErr != nil {
			return nil, addrErr
		}
		listenAddrs = append(listenAddrs, addr)
	}
	var opts []libp2p.Option
	opts = append(opts, libp2p.ListenAddrs(listenAddrs...))
	return libp2p.New(opts...)
}

const DiscoveryServiceTag = "librum-pubsub"

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

func (n *Node) Start() {

	var wg sync.WaitGroup
	h, err := n.Host()
	if err != nil {
		panic(err)
	}
	defer h.Close()

	fmt.Println("h.Addrs()", h.Addrs())

	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
	s.Start()

	ps, _ := newLibp2pPubSub(context.Background(), h)

	wg.Add(1)
	topic, _ := ps.Join("ping")

	ticker := time.NewTicker(30 * time.Second)

	go func(t *time.Ticker) {
		for {
			select {
			case <-ticker.C:
				topic.Publish(context.Background(), []byte("ss"))

			}

		}

	}(ticker)

	go func() {
		fmt.Println("Subscribe")

		s, err := topic.Subscribe()
		if err != nil {
			fmt.Println("err", err)
		}

		ctx := context.Background()
		for {

			msg, err := s.Next(ctx)
			fmt.Printf("Msg %s, recieved from Peer %s ", string(msg.Message.Data), msg.ReceivedFrom.Pretty())

			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded || err == libp2p_pubsub.ErrSubscriptionCancelled {
					fmt.Println("err", err)

				} else {
					fmt.Println("err", err)

				}

			}

		}
	}()

	fmt.Printf("Hosts ID is %s\n", h.ID())
	wg.Wait()

}

func newLibp2pPubSub(ctx context.Context, host host.Host) (*libp2p_pubsub.PubSub, error) {
	//TODO ADD Tracer

	pgParams := libp2p_pubsub.NewPeerGaterParams(
		0.33, //nolint:gomnd
		libp2p_pubsub.ScoreParameterDecay(2*time.Minute),  //nolint:gomnd
		libp2p_pubsub.ScoreParameterDecay(10*time.Minute), //nolint:gomnd
	)

	return libp2p_pubsub.NewGossipSub(
		ctx,
		host,
		libp2p_pubsub.WithPeerExchange(true),
		libp2p_pubsub.WithPeerGater(pgParams),
	)
}
