package types

import (

	"time"
	"github.com/33cn/chain33/p2pnext/manage"
	"github.com/33cn/chain33/queue"
	"github.com/33cn/chain33/types"
	core "github.com/libp2p/go-libp2p-core"
	"reflect"
)

var (
	protocolTypeMap = make(map[string]reflect.Type)
)


type IProtocol interface {
	 InitProtocol(*GlobalData)
}


func RegisterProtocolType(typeName string, proto IProtocol) {

	if proto == nil {
		panic("RegisterProtocolType, protocol is nil, msgId="+typeName)
	}
	if _, dup := protocolTypeMap[typeName]; dup {
		panic("RegisterProtocolType, protocol is nil, msgId="+typeName)
	}
	protoType := reflect.TypeOf(proto)
	if protoType.Kind() == reflect.Ptr {
	   protoType = protoType.Elem()
	}
	protocolTypeMap[typeName] = protoType
}

func init() {
	RegisterProtocolType("BaseProtocol", &BaseProtocol{})
}

type ProtocolManager struct {

	protoMap map[string]IProtocol
}


type GlobalData struct {
	ChainCfg        *types.Chain33Config
	QueueClient     queue.Client
	Host            core.Host
	StreamManager   *manage.StreamManager
	PeerInfoManager *manage.PeerInfoManager
}


// BaseProtocol store public data
type BaseProtocol struct{
	 *GlobalData
}

func (p *BaseProtocol)InitProtocol(data *GlobalData) {
	p.GlobalData = data
}

func (p *ProtocolManager)Init(data *GlobalData) {

	p.protoMap = make(map[string]IProtocol)
	//  每个P2P实例都重新分配相关的protocol结构
	for id, protocolType := range protocolTypeMap {
		protoVal := reflect.New(protocolType)
		baseValue := protoVal.Elem().FieldByName("BaseProtocol")
		//指针形式继承,需要初始化BaseProtocol结构
		if baseValue != reflect.ValueOf(nil) && baseValue.Kind() == reflect.Ptr {
			baseValue.Set(reflect.ValueOf(&BaseProtocol{}))
		}
		protocol := protoVal.Interface().(IProtocol)
		protocol.InitProtocol(data)
		p.protoMap[id] = protocol
	}

	//  每个P2P实例都重新分配相关的handler结构
	for id, handlerType := range streamHandlerTypeMap {
		handlerValue := reflect.New(handlerType)
		baseValue := handlerValue.Elem().FieldByName("BaseStreamHandler")
		//指针形式继承,需要初始化BaseStreamHandler结构
		if baseValue != reflect.ValueOf(nil) && baseValue.Kind() == reflect.Ptr {
			baseValue.Set(reflect.ValueOf(&BaseStreamHandler{}))
		}
		newHandler := handlerValue.Interface().(StreamHandler)
		protoID, msgID := decodeHandlerTypeID(id)
		newHandler.SetProtocol(p.protoMap[protoID])
		data.Host.SetStreamHandler(core.ProtocolID(msgID), (&BaseStreamHandler{child:newHandler}).HandleStream)

	}

}

func (p *BaseProtocol) NewMessageCommon(messageId, pid string, nodePubkey []byte, gossip bool) *types.MessageComm {
	return &types.MessageComm{Version: "",
		NodeId:     pid,
		NodePubKey: nodePubkey,
		Timestamp:  time.Now().Unix(),
		Id:         messageId,
		Gossip:     gossip}

}

func (p *BaseProtocol) GetChainCfg() *types.Chain33Config {

	return p.ChainCfg

}


func (p *BaseProtocol) GetQueueClient() queue.Client {

	return p.QueueClient
}

func (p *BaseProtocol) GetHost() core.Host {

	return p.Host

}

func (p *BaseProtocol)GetStreamManager() *manage.StreamManager {

	return p.StreamManager

}


func (p *BaseProtocol) GetPeerInfoManager() *manage.PeerInfoManager {
	return p.PeerInfoManager
}
