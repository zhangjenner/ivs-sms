package rtsp

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
)

// ============================================================================

// Player - 播放器数据结构定义
type Player struct {
	*Session
	Puller *Puller
	Pusher *Pusher
	cond   *sync.Cond
	queue  []*RTPPack
}

// ============================================================================

// NewPlayer - 播放器数据结构定义
func NewPlayer(session *Session, puller *Puller, pusher *Pusher) (player *Player) {
	player = &Player{
		Session: session,
		Puller:  puller,
		Pusher:  pusher,
		cond:    sync.NewCond(&sync.Mutex{}),
		queue:   make([]*RTPPack, 0),
	}
	session.StopHandles = append(session.StopHandles, func() {
		if puller != nil {
			puller.RemovePlayer(player)
		}
		if pusher != nil {
			pusher.RemovePlayer(player)
		}
		player.cond.Broadcast()
	})
	return
}

// ============================================================================

// Start - 启动播放器
func (player *Player) Start() {
	for !player.Stoped {
		// 提取RTP包
		var pack *RTPPack
		player.cond.L.Lock()
		if len(player.queue) == 0 {
			player.cond.Wait()
		}
		if len(player.queue) > 0 {
			pack = player.queue[0]
			player.queue = player.queue[1:]
		}
		player.cond.L.Unlock()
		if pack == nil {
			if !player.Stoped {
				log.Printf("player not stoped, but queue take out nil pack")
			}
			continue
		}
		// 发送RTP包
		if err := player.SendRTP(pack); err != nil {
			log.Println(err)
		}
	}
}

// ============================================================================

// QueueRTP - RTP数据入列
func (player *Player) QueueRTP(pack *RTPPack) *Player {
	if pack == nil {
		log.Printf("player queue enter nil pack, drop it")
		return player
	}
	player.cond.L.Lock()
	player.queue = append(player.queue, pack)
	player.cond.Signal()
	player.cond.L.Unlock()
	return player
}

// SendRTP - 发送RTP包
func (player *Player) SendRTP(pack *RTPPack) (err error) {
	if pack == nil {
		err = fmt.Errorf("player send rtp got nil pack")
		return
	}
	// UDP协议发送
	if player.TransType == TRANS_TYPE_UDP {
		if player.UDPClient == nil {
			err = fmt.Errorf("player use udp transport but udp client not found")
			return
		}
		err = player.UDPClient.SendRTP(pack)
		return
	}
	// TCP协议发送
	switch pack.Type {
	case RTP_TYPE_AUDIO: // 音频数据
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(player.aRTPChannel)
		player.connWLock.Lock()
		player.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		player.connRW.Write(bufLen)
		player.connRW.Write(pack.Buffer.Bytes())
		player.connRW.Flush()
		player.connWLock.Unlock()
		player.OutBytes += pack.Buffer.Len() + 4
	case RTP_TYPE_AUDIOCONTROL: // 音频控制
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(player.aRTPControlChannel)
		player.connWLock.Lock()
		player.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		player.connRW.Write(bufLen)
		player.connRW.Write(pack.Buffer.Bytes())
		player.connRW.Flush()
		player.connWLock.Unlock()
		player.OutBytes += pack.Buffer.Len() + 4
	case RTP_TYPE_VIDEO: // 视频数据
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(player.vRTPChannel)
		player.connWLock.Lock()
		player.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		player.connRW.Write(bufLen)
		player.connRW.Write(pack.Buffer.Bytes())
		player.connRW.Flush()
		player.connWLock.Unlock()
		player.OutBytes += pack.Buffer.Len() + 4
	case RTP_TYPE_VIDEOCONTROL: // 视频控制
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(player.vRTPControlChannel)
		player.connWLock.Lock()
		player.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(pack.Buffer.Len()))
		player.connRW.Write(bufLen)
		player.connRW.Write(pack.Buffer.Bytes())
		player.connRW.Flush()
		player.connWLock.Unlock()
		player.OutBytes += pack.Buffer.Len() + 4
	default:
		err = fmt.Errorf("player tcp send rtp got unkown pack type[%v]", pack.Type)
	}
	return
}
