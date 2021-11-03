package proto

import (
	"context"
	"fmt"
	"github.com/gogo/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-msgio"
	"github.com/libp2p/go-msgio/protoio"
	pb "github.com/antnest-network/ant-proto/pb"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

var log = logging.Logger("proto")

func init() {
	logging.SetLogLevel("proto", "info")
}

var streamIdleTimeout = 1 * time.Minute
var ErrReadTimeout = fmt.Errorf("timed out reading response")
var ErrCancel = fmt.Errorf("cancel request")

type AntMessenger struct {
	ctx         context.Context
	p2pHost     host.Host
	msgReq      uint32
	respChan    map[uint32]chan interface{}
	locker      sync.RWMutex
	msgHandlers map[protocol.ID]MessageHandler
}

func NewAntMessenger(ctx context.Context, p2pHost host.Host, protocols []protocol.ID) *AntMessenger {
	m := &AntMessenger{
		ctx:         ctx,
		p2pHost:     p2pHost,
		respChan:    make(map[uint32]chan interface{}),
		msgHandlers: make(map[protocol.ID]MessageHandler),
	}

	for _, pid := range protocols {
		m.p2pHost.SetStreamHandler(pid, m.handleNewStream)
	}

	return m
}

func (m *AntMessenger) Ping(ctx context.Context, to peer.ID, req *pb.Ping) (*pb.Pong, error) {
	req.Seq = m.getMsgReq()
	resp, err := m.SendRequest(ctx, ProtocolPingMessage, to, req, req.Seq)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.Pong), err
}

func (m *AntMessenger) Pong(ctx context.Context, to peer.ID, req *pb.Pong) error {
	return m.SendMessage(ctx, ProtocolPongMessage, to, req)
}

func (m *AntMessenger) PushBlock(ctx context.Context, to peer.ID, req *pb.PushBlockReq) (*pb.PushBlockResp, error) {
	req.Seq = m.getMsgReq()
	resp, err := m.SendRequest(ctx, ProtocolPushBlockMessage, to, req, req.Seq)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.PushBlockResp), err
}

func (m *AntMessenger) RespondPushBlock(ctx context.Context, to peer.ID, msg *pb.PushBlockResp) (err error) {
	return m.SendMessage(ctx, ProtocolPushBlockRespMessage, to, msg)
}

func (m *AntMessenger) MigrateBlock(ctx context.Context, to peer.ID, req *pb.MigrateBlockReq) (*pb.MigrateBlockResp, error) {
	req.Seq = m.getMsgReq()
	resp, err := m.SendRequest(ctx, ProtocolMigrateBlockMessage, to, req, req.Seq)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.MigrateBlockResp), err
}

func (m *AntMessenger) RespondMigrateBlock(ctx context.Context, to peer.ID, msg *pb.MigrateBlockResp) (err error) {
	return m.SendMessage(ctx, ProtocolMigrateBlockRespMessage, to, msg)
}

func (m *AntMessenger) SendMigrateBlockResult(ctx context.Context, to peer.ID, msg *pb.MigrateBlockResult) (err error) {
	return m.SendMessage(ctx, ProtocolMigrateBlockResultMessage, to, msg)
}

func (m *AntMessenger) SendCheque(ctx context.Context, to peer.ID, msg *pb.Cheque) (err error) {
	return m.SendMessage(ctx, ProtocolCheque, to, msg)
}

func (m *AntMessenger) SendQueen(ctx context.Context, to peer.ID, msg *pb.Queens) (err error) {
	return m.SendMessage(ctx, ProtocolQueens, to, msg)
}

func (m *AntMessenger) SendRequest(ctx context.Context, protocol protocol.ID, to peer.ID, req proto.Message, seq uint32) (resp interface{}, err error) {
	ch := make(chan interface{}, 1)
	m.locker.Lock()
	m.respChan[seq] = ch
	m.locker.Unlock()

	defer func() {
		m.locker.Lock()
		delete(m.respChan, seq)
		m.locker.Unlock()
	}()

	err = m.SendMessage(ctx, protocol, to, req)
	if err != nil {
		return nil, err
	}
	for {
		select {
		case r := <-ch:
			return r, err
		case <-ctx.Done():
			return nil, ErrCancel
		}
	}
	return nil, ErrCancel
}

func (m *AntMessenger) SendMessage(ctx context.Context, protocol protocol.ID, to peer.ID, msg proto.Message) error {
	s, err := m.p2pHost.NewStream(ctx, to, protocol)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	w := protoio.NewDelimitedWriter(s)
	return w.WriteMsg(msg)
}

func (m *AntMessenger) SetMessageHandler(id protocol.ID, h MessageHandler) {
	m.msgHandlers[id] = h
}

func (m *AntMessenger) handleNewStream(s network.Stream) {
	msgHandler := m.msgHandlers[s.Protocol()]
	if m.handleNewMessage(s, msgHandler) {
		// If we exited without error, close gracefully.
		_ = s.Close()
	} else {
		// otherwise, send an error.
		_ = s.Reset()
	}
}

func (m *AntMessenger) handleNewMessage(s network.Stream, msgHandler MessageHandler) bool {
	r := msgio.NewVarintReaderSize(s, network.MessageSizeMax)
	from := s.Conn().RemotePeer()

	timer := time.AfterFunc(streamIdleTimeout, func() { _ = s.Reset() })
	defer timer.Stop()

	for {
		msgbytes, err := r.ReadMsg()
		if err != nil {
			r.ReleaseMsg(msgbytes)
			if err == io.EOF {
				return true
			}
			return false
		}
		if err != nil {
			log.Errorf("failed to Unmarshal message: %v", err)
			continue
		}
		msgType, ok := ProtocolMessageType[s.Protocol()]
		if !ok {
			log.Errorf(" ProtocolMessageType not registered : %v", s.Protocol())
			return false
		}
		msg := reflect.New(msgType).Interface()
		err = proto.Unmarshal(msgbytes, msg.(proto.Message))
		if err != nil {
			log.Errorf("failed to parse message: %v", err)
			continue
		}
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("handler message: %v", err)
				}
			}()
			if msgHandler != nil {
				msgHandler(m.ctx, from, msg)
			} else {
				resp, ok := msg.(ResponseMessage)
				if ok {
					m.Enqueue(resp.GetSeq(), resp)
				}
			}
		}()
	}
}

func (m *AntMessenger) getMsgReq() uint32 {
	return atomic.AddUint32(&m.msgReq, 1)
}

func (m *AntMessenger) Enqueue(seq uint32, msg interface{}) {
	m.locker.RLock()
	ch, ok := m.respChan[seq]
	m.locker.RUnlock()

	if ok {
		ch <- msg
	}
}

var _ Messenger = &AntMessenger{}
