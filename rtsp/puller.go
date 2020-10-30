package rtsp

import (
	"log"
	"strings"
	"sync"
)

// ============================================================================

// Puller - 拉流器数据结构定义
type Puller struct {
	*Session

	players        map[string]*Player //SessionID <-> Player
	playersLock    sync.RWMutex
	gopCacheEnable bool
	gopCache       []*RTPPack
	gopCacheLock   sync.RWMutex
	UDPServer      *UDPServer

	cond  *sync.Cond
	queue []*RTPPack
}

// NewPuller - 新建拉流器
func NewPuller(session *Session) (puller *Puller) {
	puller = &Puller{
		Session:        session,
		players:        make(map[string]*Player),
		gopCacheEnable: true,
		gopCache:       make([]*RTPPack, 0),
		cond:           sync.NewCond(&sync.Mutex{}),
		queue:          make([]*RTPPack, 0),
	}
	session.RTPHandles = append(session.RTPHandles, func(pack *RTPPack) {
		puller.QueueRTP(pack)
	})
	session.StopHandles = append(session.StopHandles, func() {
		puller.Server.RemovePuller(puller)
		puller.cond.Broadcast()
		if puller.UDPServer != nil {
			puller.UDPServer.Stop()
			puller.UDPServer = nil
		}
	})
	return
}

// ============================================================================

// Start - 启动拉流器
func (puller *Puller) Start() {
	for !puller.Stoped {
		// 提取RTP包
		var pack *RTPPack
		puller.cond.L.Lock()
		if len(puller.queue) == 0 {
			puller.cond.Wait()
		}
		if len(puller.queue) > 0 {
			pack = puller.queue[0]
			puller.queue = puller.queue[1:]
		}
		puller.cond.L.Unlock()
		if pack == nil {
			if !puller.Stoped {
				log.Printf("puller not stoped, but queue take out nil pack")
			}
			continue
		}
		// 缓存数据帧
		if puller.gopCacheEnable {
			puller.gopCacheLock.Lock()
			if strings.EqualFold(puller.VCodec, "h264") {
				if rtp := ParseRTP(pack.Buffer.Bytes()); rtp != nil && rtp.IsKeyframeStart() {
					puller.gopCache = make([]*RTPPack, 0)
				}
				puller.gopCache = append(puller.gopCache, pack)
			} else if strings.EqualFold(puller.VCodec, "h265") {
				if rtp := ParseRTP(pack.Buffer.Bytes()); rtp != nil && rtp.IsKeyframeStartH265() {
					puller.gopCache = make([]*RTPPack, 0)
				}
				puller.gopCache = append(puller.gopCache, pack)
			}
			puller.gopCacheLock.Unlock()
		}
		// RTP包广播
		puller.BroadcastRTP(pack)
	}
}

// ============================================================================

// AddPlayer - 添加播放器
func (puller *Puller) AddPlayer(player *Player) *Puller {
	// 发送关键帧
	if puller.gopCacheEnable {
		puller.gopCacheLock.RLock()
		for _, pack := range puller.gopCache {
			player.QueueRTP(pack)
			puller.OutBytes += pack.Buffer.Len()
		}
		puller.gopCacheLock.RUnlock()
	}
	// 添加播放器
	puller.playersLock.Lock()
	if _, ok := puller.players[player.SessionID]; !ok {
		puller.players[player.SessionID] = player
		go player.Start()
		log.Printf("%v start, now player size[%d]", player, len(puller.players))
	}
	puller.playersLock.Unlock()
	return puller
}

// RemovePlayer - 删除播放器
func (puller *Puller) RemovePlayer(player *Player) *Puller {
	puller.playersLock.Lock()
	delete(puller.players, player.SessionID)
	log.Printf("%v end, now player size[%d]\n", player, len(puller.players))
	puller.playersLock.Unlock()
	return puller
}

// GetPlayers - 获取所有播放器
func (puller *Puller) GetPlayers() (players map[string]*Player) {
	players = make(map[string]*Player)
	puller.playersLock.RLock()
	for k, v := range puller.players {
		players[k] = v
	}
	puller.playersLock.RUnlock()
	return
}

// ============================================================================

// QueueRTP - RTP数据入列
func (puller *Puller) QueueRTP(pack *RTPPack) *Puller {
	puller.cond.L.Lock()
	puller.queue = append(puller.queue, pack)
	puller.cond.Signal()
	puller.cond.L.Unlock()
	return puller
}

// BroadcastRTP - RTP数据广播
func (puller *Puller) BroadcastRTP(pack *RTPPack) *Puller {
	for _, player := range puller.GetPlayers() {
		player.QueueRTP(pack)
		puller.OutBytes += pack.Buffer.Len()
	}
	return puller
}
