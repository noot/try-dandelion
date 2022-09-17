package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	//"github.com/chyeh/pubip"
	logging "github.com/ipfs/go-log"
	"github.com/urfave/cli/v2"
)

const (
	protocolID     = "/dandelion"
	defaultKeyFile = "net.key"

	testTopic = "test"
)

var log = logging.Logger("main")

var (
	flagBootnodes = "bootnodes"
	flagCount = "count"
	flagDuration = "duration"

	app = &cli.App{
		Name:                 "try-dandelion",
		Usage:                "try out libp2p nodes with dandelion++ gossip",
		Action:               run,
		EnableBashCompletion: true,
		Suggest:              true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagBootnodes,
				Usage: "comma-separated list of bootnodes",
				Value: "",
			},
			&cli.UintFlag{
				Name:  flagCount,
				Usage: "number of nodes to run",
				Value: 10,
			},
			&cli.UintFlag{
				Name:  flagDuration,
				Usage: "length of time to run simulation in seconds",
				Value: 120,
			},
		},
	}
)

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	_ = logging.SetLogLevel("main", "info")

	const basePort = 5000

	bootnodes := []peer.AddrInfo{}
	hosts := []*host{}

	count := int(c.Uint(flagCount))
	mlt := newMessageLatencyTracker(count)
	go mlt.trackRecieved()

	for i := 0; i < count; i++ {
		log.Infof("starting node %d", i)
		cfg := &config{
			Ctx: context.Background(),
			Port: uint16(basePort + i),
			Bootnodes: bootnodes,
			Index: i,
			Tracker: mlt,
		}

		h, err := NewHost(cfg)
		if err != nil {
			return err
		}

		err = h.Start()
		if err != nil {
			return err
		}

		time.Sleep(time.Second)
		log.Infof("node %d started: %s", i, h.addrInfo())
		bootnodes = append(bootnodes, h.addrInfo())
		hosts = append(hosts, h)
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%ds", c.Uint(flagDuration)))
	if err != nil {
		return err
	}
	<-time.After(duration)

	for _, h := range hosts {
		err := h.Stop()
		if err != nil {
			return err
		}
	}

	mlt.log()
	return nil
}

type config struct {
	Ctx         context.Context
	Port        uint16
	KeyFile     string
	Bootnodes   []peer.AddrInfo
	Index int
	Tracker *messageLatencyTracker
}

type host struct {
	ctx        context.Context
	cancel     context.CancelFunc
	h libp2phost.Host
	bootnodes []peer.AddrInfo
	topics map[string]*pubsub.Topic
	tracker *messageLatencyTracker
}

func NewHost(cfg *config) (*host, error) {
	if cfg.KeyFile == "" {
		cfg.KeyFile = fmt.Sprintf("node-%d.key", cfg.Index)
	}

	key, err := loadKey(cfg.KeyFile)
	if err != nil {
		log.Infof("failed to load libp2p key, generating key %s...", cfg.KeyFile)
		key, err = generateKey(0, cfg.KeyFile)
		if err != nil {
			return nil, err
		}
	}

	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Port))
	if err != nil {
		return nil, err
	}

	// var externalAddr ma.Multiaddr
	// ip, err := pubip.Get()
	// if err != nil {
	// 	log.Warnf("failed to get public IP error: %v", err)
	// } else {
	// 	log.Debugf("got public IP address %s", ip)
	// 	externalAddr, err = ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ip, cfg.Port))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	opts := []libp2p.Option{
		libp2p.ListenAddrs(addr),
		libp2p.Identity(key),
		libp2p.NATPortMap(),
		// libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
		// 	return append(addrs, externalAddr)
		// }),
	}

	// // format bootnodes
	// bns, err := stringsToAddrInfos(cfg.Bootnodes)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to format bootnodes: %w", err)
	// }

	// create libp2p host instance
	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	psOpts := []pubsub.Option{
		pubsub.WithMessageIdFn(messageIDFn),
	}

	ps, err := pubsub.NewGossipSub(cfg.Ctx, h, psOpts...)
	if err != nil {
		return nil, err
	}

	topic, err := ps.Join(testTopic)
	if err != nil {
		return nil, err
	}

	topics := map[string]*pubsub.Topic{
		testTopic: topic,
	}

	ourCtx, cancel := context.WithCancel(cfg.Ctx)
	return &host{
		ctx: ourCtx,
		cancel: cancel,
		h: h,
		bootnodes: cfg.Bootnodes,
		topics: topics,
		tracker: cfg.Tracker,
	}, nil
}

func messageIDFn(pmsg *pb.Message) string {
	return getMessageIDFromData(pmsg.Data)
}

func getMessageIDFromData(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (h *host) addrInfo() peer.AddrInfo {
	return peer.AddrInfo{
		ID:    h.h.ID(),
		Addrs: h.h.Addrs(),
	}
}

func (h *host) Start() error {
	err := h.bootstrap()
	if err != nil {
		return err
	}

	for topic := range h.topics {
		go func() {
			err := h.receive(topic)
			if err != nil {
				log.Warnf("receive loop exiting: %s", err)
				return
			}
		}()
	}

	// TODO: make random
	ticker := time.NewTicker(time.Second * 10)
	go func() {
		for {
			select {
			case <-h.ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				go func() {
					err := h.publishRandomMessages()
					if err != nil {
						log.Warnf("send loop exiting: %s", err)
						return	
					}
				}()
			}
		}
	}()

	return nil
}

func (h *host) Stop() error {
	h.cancel()
	if err := h.h.Close(); err != nil {
		return fmt.Errorf("failed to close libp2p host: %w", err)
	}
	return nil
}

func (h *host) publishRandomMessages() error {
	const messageLength = 16
	for topic := range h.topics {
		msg := make([]byte, messageLength)
		_, err := rand.Read(msg)
		if err != nil {
			return err
		}

		err = h.publish(topic, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

// bootstrap connects the host to the configured bootnodes
func (h *host) bootstrap() error {
	failed := 0
	for _, addrInfo := range h.bootnodes {
		log.Debugf("bootstrapping to peer: peer=%s", addrInfo.ID)
		err := h.h.Connect(h.ctx, addrInfo)
		if err != nil {
			log.Debugf("failed to bootstrap to peer: err=%s", err)
			failed++
		}
	}

	if failed == len(h.bootnodes) && len(h.bootnodes) != 0 {
		return errFailedToBootstrap
	}

	return nil
}

func (h *host) publish(topic string, msg []byte) error {
	t, has := h.topics[topic]
	if !has {
		return errNoTopic
	}

	id := getMessageIDFromData(msg)
	log.Infof("sent message on topic %s: id %s", topic, id)
	h.tracker.logSent(id)
	return t.Publish(h.ctx, msg)
}

func (h *host) receive(topic string) error {
	t, has := h.topics[topic]
	if !has {
		return errNoTopic
	}

	sub, err := t.Subscribe()
	if err != nil {
		return err
	}

	for {
		msg, err := sub.Next(h.ctx)
		if err != nil {
			return err
		}

		log.Infof("got message on topic %s: id %s", topic, msg.ID)
		h.tracker.ch <- msg.ID
	}

	return nil
}