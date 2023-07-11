package relay

import (
	"fmt"
	"github.com/emc-protocol/edge-matrix/network/grpc"
	"github.com/emc-protocol/edge-matrix/relay/alive"
	"github.com/emc-protocol/edge-matrix/relay/proto"
	"github.com/emc-protocol/edge-matrix/secrets"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p"
	kb "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
	rawGrpc "google.golang.org/grpc"
	"log"
	"sync"
	"time"
)

const (
	defaultBucketSize = 256
)

// PeerConnInfo holds the connection information about the peer
type PeerConnInfo struct {
	Info peer.AddrInfo

	connDirections  map[network.Direction]bool
	protocolStreams map[string]*rawGrpc.ClientConn
}

// addProtocolStream adds a protocol stream
func (pci *PeerConnInfo) addProtocolStream(protocol string, stream *rawGrpc.ClientConn) {
	pci.protocolStreams[protocol] = stream
}

// removeProtocolStream removes and closes a protocol stream
func (pci *PeerConnInfo) removeProtocolStream(protocol string) error {
	stream, ok := pci.protocolStreams[protocol]
	if !ok {
		return nil
	}

	delete(pci.protocolStreams, protocol)

	if stream != nil {
		return stream.Close()
	}

	return nil
}

// getProtocolStream fetches the protocol stream, if any
func (pci *PeerConnInfo) getProtocolStream(protocol string) *rawGrpc.ClientConn {
	return pci.protocolStreams[protocol]
}

type RelayServer struct {
	logger hclog.Logger // the logger

	protocols     map[string]Protocol // supported protocols
	protocolsLock sync.Mutex          // lock for the supported protocols map

	host host.Host // the libp2p host reference
}

func (s *RelayServer) GetHost() host.Host {
	return s.host
}

type Protocol interface {
	Client(network.Stream) *rawGrpc.ClientConn
	Handler() func(network.Stream)
}

func (s *RelayServer) RegisterProtocol(id string, p Protocol) {
	s.protocolsLock.Lock()
	defer s.protocolsLock.Unlock()

	s.protocols[id] = p
	s.wrapStream(id, p.Handler())
}

func (s *RelayServer) wrapStream(id string, handle func(network.Stream)) {
	s.host.SetStreamHandler(protocol.ID(id), func(stream network.Stream) {
		peerID := stream.Conn().RemotePeer()
		s.logger.Debug("open stream", "protocol", id, "peer", peerID)

		handle(stream)
	})
}

// setupAlive Sets up the live service for the node
func (s *RelayServer) SetupAliveService() error {
	// Set up a fresh routing table
	keyID := kb.ConvertPeerID(s.host.ID())

	routingTable, err := kb.NewRoutingTable(
		defaultBucketSize,
		keyID,
		time.Minute,
		s.host.Peerstore(),
		10*time.Second,
		nil,
	)
	if err != nil {
		return err
	}

	// Set the PeerAdded event handler
	routingTable.PeerAdded = func(p peer.ID) {
		//info := s.host.Peerstore().PeerInfo(p)
		//s.addToDialQueue(&info, common.PriorityRandomDial)
	}

	// Set the PeerRemoved event handler
	routingTable.PeerRemoved = func(p peer.ID) {
		//s.dialQueue.DeleteTask(p)
	}

	// Create an instance of the alive service
	aliveService := alive.NewAliveService(
		routingTable,
		s.logger,
	)

	// Register the actual alive service as a valid protocol
	s.registerAliveService(aliveService)

	return nil
}

// registerDiscoveryService registers the discovery protocol to be available
func (s *RelayServer) registerAliveService(aliveService *alive.AliveService) {
	grpcStream := grpc.NewGrpcStream()
	proto.RegisterAliveServer(grpcStream.GrpcServer(), aliveService)
	grpcStream.Serve()

	s.RegisterProtocol(alive.EdgeAliveProto, grpcStream)
}

// NewRelayServer returns a new instance of the relay server
func NewRelayServer(logger hclog.Logger, secretsManager secrets.SecretsManager, relayListenAddr multiaddr.Multiaddr) (*RelayServer, error) {
	logger = logger.Named("network")

	key, err := setupLibp2pKey(secretsManager)
	if err != nil {
		return nil, err
	}

	relayHost, err := libp2p.New(
		libp2p.Security(noise.ID, noise.New),
		libp2p.ListenAddrs(relayListenAddr),
		libp2p.Identity(key),
	)
	if err != nil {
		log.Printf("Failed to create relay server host: %v", err)
		return nil, err
	}
	_, err = relay.New(relayHost)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to instantiate the relay: %v", err))
	}

	srv := &RelayServer{
		logger:    logger,
		host:      relayHost,
		protocols: map[string]Protocol{},
	}

	return srv, nil
}