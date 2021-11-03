package proto

import (
	"context"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pb "github.com/antnest-network/ant-proto/pb"
	"reflect"
)

const (
	Success = 0
	Failure = 1
)

var (
	ProtocolPingMessage protocol.ID = "/ant/ping/1.0.0"
	ProtocolPongMessage protocol.ID = "/ant/pong/1.0.0"

	ProtocolPushBlockMessage     protocol.ID = "/ant/push_block/1.0.0"
	ProtocolPushBlockRespMessage protocol.ID = "/ant/push_block_resp/1.0.0"

	ProtocolMigrateBlockMessage       protocol.ID = "/ant/migrate_block/1.0.0"
	ProtocolMigrateBlockRespMessage   protocol.ID = "/ant/migrate_block_resp/1.0.0"
	ProtocolMigrateBlockResultMessage protocol.ID = "/ant/migrate_block_result/1.0.0"

	ProtocolCheque protocol.ID = "/ant/cheque/1.0.0"
	ProtocolQueens protocol.ID = "/ant/queens/1.0.0"

	Protocols = []protocol.ID{
		ProtocolPingMessage,
		ProtocolPongMessage,
		ProtocolPushBlockMessage,
		ProtocolPushBlockRespMessage,
		ProtocolMigrateBlockMessage,
		ProtocolMigrateBlockRespMessage,
		ProtocolMigrateBlockResultMessage,
		ProtocolCheque,
		ProtocolQueens,
	}

	ProtocolMessageType = map[protocol.ID]reflect.Type{
		ProtocolPingMessage:               reflect.TypeOf(pb.Ping{}),
		ProtocolPongMessage:               reflect.TypeOf(pb.Pong{}),
		ProtocolPushBlockMessage:          reflect.TypeOf(pb.PushBlockReq{}),
		ProtocolPushBlockRespMessage:      reflect.TypeOf(pb.PushBlockResp{}),
		ProtocolMigrateBlockMessage:       reflect.TypeOf(pb.MigrateBlockReq{}),
		ProtocolMigrateBlockRespMessage:   reflect.TypeOf(pb.MigrateBlockResp{}),
		ProtocolMigrateBlockResultMessage: reflect.TypeOf(pb.MigrateBlockResult{}),
		ProtocolCheque:                    reflect.TypeOf(pb.Cheque{}),
		ProtocolQueens:                    reflect.TypeOf(pb.Queens{}),
	}
)

type MessageHandler func(ctx context.Context, from peer.ID, msg interface{})

type Messenger interface {
	Ping(ctx context.Context, to peer.ID, msg *pb.Ping) (resp *pb.Pong, err error)

	Pong(ctx context.Context, to peer.ID, msg *pb.Pong) (err error)

	PushBlock(ctx context.Context, to peer.ID, msg *pb.PushBlockReq) (resp *pb.PushBlockResp, err error)

	RespondPushBlock(ctx context.Context, to peer.ID, msg *pb.PushBlockResp) (err error)

	MigrateBlock(ctx context.Context, to peer.ID, msg *pb.MigrateBlockReq) (resp *pb.MigrateBlockResp, err error)

	RespondMigrateBlock(ctx context.Context, to peer.ID, msg *pb.MigrateBlockResp) (err error)

	SendMigrateBlockResult(ctx context.Context, to peer.ID, msg *pb.MigrateBlockResult) (err error)

	SendCheque(ctx context.Context, to peer.ID, msg *pb.Cheque) (err error)

	SendQueen(ctx context.Context, to peer.ID, msg *pb.Queens) (err error)

	SetMessageHandler(proto protocol.ID, h MessageHandler)
}

type ResponseMessage interface {
	GetSeq() uint32
}

type PeerHandler interface {
	HandleNewPeer(peer.AddrInfo) error
}

type PeerListener interface {
	SetPeerHandler(h PeerHandler)
}
