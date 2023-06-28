package network

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/multiformats/go-multiaddr"
	"math/big"
	"time"

	"github.com/emc-protocol/edge-matrix/network/common"
	"github.com/emc-protocol/edge-matrix/network/discovery"
	"github.com/emc-protocol/edge-matrix/network/grpc"
	"github.com/emc-protocol/edge-matrix/network/proto"
	kb "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	rawGrpc "google.golang.org/grpc"
)

const (
	topicNameV1 = "route_table/0.2"
)

// GetRandomBootnode fetches a random bootnode that's currently
// NOT connected, if any
func (s *Server) GetRandomBootnode() *peer.AddrInfo {
	nonConnectedNodes := make([]*peer.AddrInfo, 0)

	for _, v := range s.bootnodes.getBootnodes() {
		if !s.hasPeer(v.ID) {
			nonConnectedNodes = append(nonConnectedNodes, v)
		}
	}

	if len(nonConnectedNodes) > 0 {
		randNum, _ := rand.Int(rand.Reader, big.NewInt(int64(len(nonConnectedNodes))))

		return nonConnectedNodes[randNum.Int64()]
	}

	return nil
}

// GetBootnodeConnCount fetches the number of active bootnode connections [Thread safe]
func (s *Server) GetBootnodeConnCount() int64 {
	return s.bootnodes.getBootnodeConnCount()
}

// getProtoStream returns an active protocol stream if present, otherwise
// it returns nil
func (s *Server) getProtoStream(protocol string, peerID peer.ID) *rawGrpc.ClientConn {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	connectionInfo, ok := s.peers[peerID]
	if !ok {
		return nil
	}

	return connectionInfo.getProtocolStream(protocol)
}

// NewDiscoveryClient returns a new or existing discovery service client connection
func (s *Server) NewDiscoveryClient(peerID peer.ID) (proto.DiscoveryClient, error) {
	// Temporary dials are never added to the peer store,
	// so they have a special status when doing discovery
	isTemporaryDial := s.IsTemporaryDial(peerID)

	// Check if there is a peer connection at this point in time,
	// as there might have been a disconnection previously
	if !s.IsConnected(peerID) && !isTemporaryDial {
		return nil, fmt.Errorf("could not initialize new discovery client - peer [%s] not connected",
			peerID.String())
	}

	// Check if there is an active stream connection already
	if protoStream := s.getProtoStream(s.discProto, peerID); protoStream != nil {
		return proto.NewDiscoveryClient(protoStream), nil
	}

	// Create a new stream connection and return it
	protoStream, err := s.NewProtoConnection(s.discProto, peerID)
	if err != nil {
		return nil, err
	}

	// Discovery protocol streams should be saved,
	// since they are referenced later on,
	// if they are not temporary
	if !isTemporaryDial {
		s.SaveProtocolStream(s.discProto, protoStream, peerID)
	}

	return proto.NewDiscoveryClient(protoStream), nil
}

// SaveProtocolStream saves the protocol stream to the peer
// protocol stream reference [Thread safe]
func (s *Server) SaveProtocolStream(
	protocol string,
	stream *rawGrpc.ClientConn,
	peerID peer.ID,
) {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	connectionInfo, ok := s.peers[peerID]
	if !ok {
		s.logger.Warn(
			fmt.Sprintf(
				"Attempted to save protocol %s stream for non-existing peer %s",
				protocol,
				peerID,
			),
		)

		return
	}

	connectionInfo.addProtocolStream(protocol, stream)
}

// CloseProtocolStream closes a protocol stream to the specified peer
func (s *Server) CloseProtocolStream(protocol string, peerID peer.ID) error {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	connectionInfo, ok := s.peers[peerID]
	if !ok {
		return nil
	}

	return connectionInfo.removeProtocolStream(protocol)
}

// AddToPeerStore adds peer information to the node's peer store
func (s *Server) AddToPeerStore(peerInfo *peer.AddrInfo) {
	s.host.Peerstore().AddAddr(peerInfo.ID, peerInfo.Addrs[0], peerstore.PermanentAddrTTL)

}

func (s *Server) addPeerUpdateInfo(from peer.ID, id peer.ID, addrInfo peer.AddrInfo, gossip bool) bool {
	s.updatePeersLock.Lock()
	defer s.updatePeersLock.Unlock()

	s.logger.Info("addPeerUpdateInfo", "addrInfo", addrInfo.String(), "from", from.String(), "gossip", gossip)

	updateInfo, updateInfoExists := s.updatePeers[id]

	// Check if the peer update info is already initialized
	needToPublish := false
	if !updateInfoExists {
		// Create a new record for the connection info
		updateInfo = &PeerUpdateInfo{
			Info:        addrInfo,
			UpdateTime:  time.Now(),
			From:        from.String(),
			PublishTime: time.Now(),
		}
		s.updatePeers[id] = updateInfo
		needToPublish = true
	} else {
		if s.updatePeers[id].PublishTime.After(s.updatePeers[id].UpdateTime.Add(peerstore.TempAddrTTL)) {
			s.updatePeers[id].PublishTime = time.Now()
			needToPublish = true
		}
		if s.updatePeers[id].PublishTime.After(s.updatePeers[id].UpdateTime.Add(peerstore.TempAddrTTL)) {
			s.updatePeers[id].Info = addrInfo
			s.updatePeers[id].From = from.String()
			s.updatePeers[id].UpdateTime = time.Now()
			needToPublish = true
		}
	}

	// broadcast new addr
	if !gossip && needToPublish && s.rtTopic != nil {
		filteredPeers := make([]string, 0)
		filteredPeers = append(filteredPeers, common.AddrInfoToString(&addrInfo))
		s.rtTopic.Publish(&proto.PeerInfo{From: s.host.ID().String(), Nodes: filteredPeers})
		s.logger.Debug("AddToPeerStore", "Publish", filteredPeers)
	}

	return false
}

func (s *Server) AddAddr(p peer.ID, addr multiaddr.Multiaddr) {
	s.host.Peerstore().AddAddr(p, addr, peerstore.PermanentAddrTTL)

	addrInfoString := addr.String() + "/p2p/" + p.String()
	addrInfo, err := common.StringToAddrInfo(addrInfoString)
	if err != nil {
		s.logger.Error("AddAddr", "NodeId", p)
		return
	}
	s.addPeerUpdateInfo(s.host.ID(), p, *addrInfo, false)
}

func (s *Server) handlePeerStoreUpdateGossip(obj interface{}, from peer.ID) {
	peerInfo, ok := obj.(*proto.PeerInfo)
	if !ok {
		s.logger.Error("failed to cast gossiped message to proto.PeerInfo")
		return
	}
	if from.String() == s.host.ID().String() {
		return
	}

	nodes := peerInfo.Nodes
	for _, rawAddr := range nodes {
		node, err := common.StringToAddrInfo(rawAddr)
		if err != nil {
			s.logger.Error("handlePeerStoreUpdateGossip", "err", fmt.Sprintf("failed to parse rawAddr %s: %w", rawAddr, err))
			continue
		}
		s.host.Peerstore().AddAddr(node.ID, node.Addrs[0], peerstore.PermanentAddrTTL)
		s.logger.Info("handlePeerStoreUpdateGossip", "from", from, "node", node.String())

		s.addPeerUpdateInfo(from, node.ID, *node, true)
	}
}

func (s *Server) StartPeerStoreUpdateGossip() error {
	topic, err := s.NewTopic(topicNameV1, &proto.PeerInfo{})
	if err != nil {
		return err
	}

	if err := topic.Subscribe(s.handlePeerStoreUpdateGossip); err != nil {
		return fmt.Errorf("unable to subscribe to gossip topic, %w", err)
	}

	s.rtTopic = topic
	s.logger.Info("StartPeerStoreUpdateGossip")

	return nil
}

// RemoveFromPeerStore removes peer information from the node's peer store
func (s *Server) RemoveFromPeerStore(peerInfo *peer.AddrInfo) {
	s.host.Peerstore().RemovePeer(peerInfo.ID)
}

// GetPeerInfo fetches the information of a peer
func (s *Server) GetPeerInfo(peerID peer.ID) *peer.AddrInfo {
	info := s.host.Peerstore().PeerInfo(peerID)

	return &info
}

// GetRandomPeer fetches a random peer from the peers list
func (s *Server) GetRandomPeer() *peer.ID {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()

	if len(s.peers) < 1 {
		return nil
	}

	randNum, _ := rand.Int(
		rand.Reader,
		big.NewInt(int64(len(s.peers))),
	)

	randomPeerIndx := int(randNum.Int64())

	counter := 0
	for peerID := range s.peers {
		if randomPeerIndx == counter {
			return &peerID
		}

		counter++
	}

	return nil
}

// FetchOrSetTemporaryDial loads the temporary status of a peer connection, and
// sets a new value [Thread safe]
func (s *Server) FetchOrSetTemporaryDial(peerID peer.ID, newValue bool) bool {
	_, loaded := s.temporaryDials.LoadOrStore(peerID, newValue)

	return loaded
}

// RemoveTemporaryDial removes a peer connection as temporary [Thread safe]
func (s *Server) RemoveTemporaryDial(peerID peer.ID) {
	s.temporaryDials.Delete(peerID)
}

// setupDiscovery Sets up the discovery service for the node
func (s *Server) setupDiscovery() error {
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
		info := s.host.Peerstore().PeerInfo(p)
		s.addToDialQueue(&info, common.PriorityRandomDial)
	}

	// Set the PeerRemoved event handler
	routingTable.PeerRemoved = func(p peer.ID) {
		s.dialQueue.DeleteTask(p)
	}

	// Create an instance of the discovery service
	discoveryService := discovery.NewDiscoveryService(
		s,
		routingTable,
		s.logger,
	)

	// Register a network event handler
	if subscribeErr := s.SubscribeFn(context.Background(), discoveryService.HandleNetworkEvent); subscribeErr != nil {
		return fmt.Errorf("unable to subscribe to network events, %w", subscribeErr)
	}

	// Register the actual discovery service as a valid protocol
	s.registerDiscoveryService(discoveryService)

	// Make sure the discovery service has the bootnodes in its routing table,
	// and instantiates connections to them
	discoveryService.ConnectToBootnodes(s.bootnodes.getBootnodes())

	// Start the discovery service
	discoveryService.Start()

	// Set the discovery service reference
	s.discovery = discoveryService

	return nil
}

func (s *Server) TemporaryDialPeer(peerAddrInfo *peer.AddrInfo) {
	s.logger.Debug("creating new temporary dial to peer", "peer", peerAddrInfo.ID)
	s.addToDialQueue(peerAddrInfo, common.PriorityRandomDial)
}

// registerDiscoveryService registers the discovery protocol to be available
func (s *Server) registerDiscoveryService(discovery *discovery.DiscoveryService) {
	grpcStream := grpc.NewGrpcStream()
	proto.RegisterDiscoveryServer(grpcStream.GrpcServer(), discovery)
	grpcStream.Serve()

	s.RegisterProtocol(s.discProto, grpcStream)
}
