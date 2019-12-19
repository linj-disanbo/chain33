package peer

import (
	"time"

	"github.com/33cn/chain33/common/log/log15"
	prototypes "github.com/33cn/chain33/p2pnext/protocol/types"
	"github.com/33cn/chain33/queue"
	"github.com/33cn/chain33/types"
	uuid "github.com/google/uuid"
	core "github.com/libp2p/go-libp2p-core"
	net "github.com/libp2p/go-libp2p-core/network"
)

const (
	protoTypeID  = "PeerProtocolType"
	PeerInfoReq  = "/chain33/peerinfoReq/1.0.0"
	PeerInfoResp = "/chain33/peerinfoResp/1.0.0"
)

var log = log15.New("module", "p2p.peer")

func init() {
	prototypes.RegisterProtocolType(protoTypeID, &PeerInfoProtol{})
	var hander = new(PeerInfoHandler)
	prototypes.RegisterStreamHandlerType(protoTypeID, PeerInfoReq, hander)
	prototypes.RegisterStreamHandlerType(protoTypeID, PeerInfoResp, hander)

}

//type Istream
type PeerInfoProtol struct {
	prototypes.BaseProtocol
	p2pCfg   *types.P2P
	requests map[string]*types.MessagePeerInfoReq // used to access request data from response handlers
}

func (p *PeerInfoProtol) InitProtocol(data *prototypes.GlobalData) {
	p.requests = make(map[string]*types.MessagePeerInfoReq)
	p.GlobalData = data
	p.p2pCfg = data.ChainCfg.GetModuleConfig().P2P
	go p.ManagePeerInfo()
	prototypes.RegisterEventHandler(types.EventPeerInfo, p.handleEvent)

}

func (p *PeerInfoProtol) OnResp(peerinfo *types.P2PPeerInfo, s net.Stream) {
	peerId := s.Conn().RemotePeer().Pretty()
	p.GetPeerInfoManager().Store(peerId, peerinfo)
	log.Debug("OnResp Received ping response ", "from", s.Conn().LocalPeer(), "to", s.Conn().RemotePeer())

}

func (p *PeerInfoProtol) getLoacalPeerInfo() *types.P2PPeerInfo {
	client := p.GetQueueClient()
	var peerinfo types.P2PPeerInfo

	msg := client.NewMessage("mempool", types.EventGetMempoolSize, nil)
	err := client.SendTimeout(msg, true, time.Second*10)
	if err != nil {
		log.Error("GetPeerInfo mempool", "Error", err.Error())
	}
	resp, err := client.WaitTimeout(msg, time.Second*10)
	if err != nil {
		log.Error("GetPeerInfo EventGetLastHeader", "Error", err.Error())

	} else {
		meminfo := resp.GetData().(*types.MempoolSize)
		peerinfo.MempoolSize = int32(meminfo.GetSize())
	}

	msg = client.NewMessage("blockchain", types.EventGetLastHeader, nil)
	err = client.SendTimeout(msg, true, time.Minute)
	if err != nil {
		log.Error("GetPeerInfo EventGetLastHeader", "Error", err.Error())
		goto Jump

	}
	resp, err = client.WaitTimeout(msg, time.Second*10)
	if err != nil {
		log.Error("GetPeerInfo EventGetLastHeader", "Error", err.Error())

		goto Jump

	}
Jump:
	header := resp.GetData().(*types.Header)
	peerinfo.Header = header
	peerinfo.Name = p.Host.ID().Pretty()

	peerinfo.Addr = p.Host.Addrs()[0].String()
	return &peerinfo
}
func (p *PeerInfoProtol) ManagePeerInfo() {
	p.PeerInfo()
	for {
		select {
		case <-time.Tick(time.Second * 20):
			p.PeerInfo()
		}
	}

}

//p2pserver 端接收处理事件
func (p *PeerInfoProtol) OnReq(s net.Stream) {

	peerinfo := p.getLoacalPeerInfo()
	peerID := p.GetHost().ID()
	pubkey, _ := p.GetHost().Peerstore().PubKey(peerID).Bytes()

	resp := &types.MessagePeerInfoResp{MessageData: p.NewMessageCommon(uuid.New().String(), peerID.Pretty(), pubkey, false),
		Message: peerinfo}
	s.SetProtocol(PeerInfoResp)
	ok := p.StreamManager.SendProtoMessage(resp, s)

	if ok {
		log.Info(" OnReq", "localPeer", s.Conn().LocalPeer().String(), "remotePeer", s.Conn().RemotePeer().String())
	}

}

// PeerInfo 向对方节点请求peerInfo信息
func (p *PeerInfoProtol) PeerInfo() {

	pid := p.GetHost().ID()
	pubkey, _ := p.GetHost().Peerstore().PubKey(pid).Bytes()
	for _, s := range p.GetStreamManager().FetchStreams() {
		log.Info("peerInfo", "s.Proto", s.Protocol())
		req := &types.MessagePeerInfoReq{MessageData: p.NewMessageCommon(uuid.New().String(), pid.Pretty(), pubkey, false)}
		s.SetProtocol(PeerInfoReq)
		ok := p.StreamManager.SendProtoMessage(req, s)
		if !ok {
			return
		}

		// store ref request so response handler has access to it
		p.requests[req.MessageData.Id] = req
		log.Info("PeerInfo", "sendRequst", pid.Pretty())
	}

}

//接收chain33其他模块发来的请求消息
func (p *PeerInfoProtol) handleEvent(msg *queue.Message) {

	_, ok := msg.GetData().(*types.P2PGetPeerInfo)
	if !ok {
		return
	}
	peers := p.PeerInfoManager.FetchPeers()

	var peer types.Peer
	peerinfo := p.getLoacalPeerInfo()
	p.PeerInfoManager.Copy(&peer, peerinfo)
	peer.Self = true
	peers = append(peers, &peer)
	msg.Reply(p.GetQueueClient().NewMessage("blockchain", types.EventPeerList, &types.PeerList{Peers: peers}))

}

type PeerInfoHandler struct {
	prototypes.BaseStreamHandler
}

//Handle 处理请求
func (h *PeerInfoHandler) Handle(req []byte, stream core.Stream) {
	log.Info("peerInfo", "hander", "process")
	protocol := h.GetProtocol().(*PeerInfoProtol)

	//解析处理
	log.Info("PeerInfo Handler", "stream proto", stream.Protocol())
	if stream.Protocol() == PeerInfoReq {
		var data types.MessagePeerInfoReq
		err := types.Decode(req, &data)
		if err != nil {
			return
		}

		protocol.OnReq(stream)
		return
	}

	var data types.MessagePeerInfoResp
	err := types.Decode(req, &data)
	if err != nil {
		return
	}

	protocol.OnResp(data.Message, stream)

	return
}

func (p *PeerInfoHandler) VerifyRequest(data []byte) bool {

	return true
}
