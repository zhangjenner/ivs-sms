package rtsp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
	"time"
)

// ============================================================================

// Server - RTSP服务定义
type Server struct {
	*ServerCfg
	Stoped        bool                 //是否停止
	TCPListener   *net.TCPListener     //TCP端口监听器
	pullers       map[string]*Puller   //管理的拉流器：Path <-> Puller
	pushers       map[string]*Pusher   //管理的推流器：Path <-> Pusher
	recorders     map[string]*Recorder //管理的录像器：Path <-> Recorder
	pullersLock   sync.RWMutex         //拉流器访问锁
	pushersLock   sync.RWMutex         //推流器访问锁
	recordersLock sync.RWMutex         //录像器访问锁
}

// ServerCfg - RTSP服务配置
type ServerCfg struct {
	TCPPort     int    //TCP监听端口
	NetBufSize  int    //网络缓存大小
	NetTimeout  int    //网络超时时间
	FfmpegPath  string //ffmpeg路径
	SavePath    string //录像存储路径
	DurationSec int    //切片文件时长(秒)
}

// Instance - RTSP服务单例对象
var Instance *Server = &Server{
	Stoped:    true,
	pullers:   make(map[string]*Puller),
	pushers:   make(map[string]*Pusher),
	recorders: make(map[string]*Recorder),
}

// GetServer - 获取RTSP服务单例对象
func GetServer() *Server {
	return Instance
}

// ============================================================================

// CfgServer - 配置RTSP服务参数
func (server *Server) CfgServer(cfg *ServerCfg) (err error) {
	if !server.Stoped {
		return errors.New("The RTSP server is runing, can't setting")
	}
	server.ServerCfg = cfg
	return nil
}

// Start - 启动RSTP服务
func (server *Server) Start() (err error) {
	// 参数判断
	if !server.Stoped {
		return errors.New("The RTSP server is runing, can't start again")
	}
	if server.TCPPort <= 0 || server.NetBufSize <= 0 {
		return errors.New("The RTSP server TCPPort or NetBufSize is wrong")
	}
	// 获取监听端口
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", server.TCPPort))
	if err != nil {
		log.Println(err)
		return
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Println(err)
		return
	}
	// 启动拉流监视
	server.Stoped = false
	go server.MonPullers()
	// 启动推流、播放监听
	server.TCPListener = listener
	for !server.Stoped {
		conn, err := server.TCPListener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}
		if err := conn.SetReadBuffer(server.NetBufSize); err != nil {
			log.Printf("rtsp server conn set read buffer error, %v", err)
		}
		if err := conn.SetWriteBuffer(server.NetBufSize); err != nil {
			log.Printf("rtsp server conn set write buffer error, %v", err)
		}
		session := NewSession(server, conn, SESSION_TYPE_UNKNOWN, "", "")
		go session.Start()
	}
	return
}

// Stop - 停止RSTP服务
func (server *Server) Stop() {
	log.Println("rtsp server stop on", server.TCPPort)
	server.Stoped = true
	if server.TCPListener != nil {
		server.TCPListener.Close()
		server.TCPListener = nil
	}
	server.pullersLock.Lock()
	server.pullers = make(map[string]*Puller)
	server.pullersLock.Unlock()
	server.pushersLock.Lock()
	server.pushers = make(map[string]*Pusher)
	server.pushersLock.Unlock()
}

// ============================================================================

// MonPullers - 监视拉流器
func (server *Server) MonPullers() {
	for !server.Stoped {
		// var allPullers []models.Puller
		// if err := db.SQLite.Find(&allPullers).Error; err != nil {
		// 	log.Println("[ERROR]", err)
		// } else {
		// 	for _, puller := range allPullers {
		// 		if p := server.GetPuller(puller.Path); p == nil {
		// 			err := server.AddSession(SESSION_TYPE_PULLER, puller.Path[1:], puller.Addr)
		// 			if err != nil {
		// 				log.Println("[ERROR]", err)
		// 			}
		// 		}
		// 	}
		// }
		time.Sleep(10 * time.Second)
	}
}

// AddSession - 添加主动会话
func (server *Server) AddSession(stype SessionType, name, addr string) error {
	// 建立TCP连接
	url, err := url.Parse(addr)
	if err != nil {
		return err
	}
	ipaddr, err := net.ResolveTCPAddr("tcp", url.Host)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP("tcp", nil, ipaddr)
	if err != nil {
		return err
	}
	// 设置连接参数
	if err := conn.SetReadBuffer(server.NetBufSize); err != nil {
		return err
	}
	if err := conn.SetWriteBuffer(server.NetBufSize); err != nil {
		return err
	}
	// 启动会话
	session := NewSession(server, conn, stype, name, addr)
	go session.Start()
	session.Trigger()
	return nil
}

// ============================================================================

// AddPuller - 添加拉流器
func (server *Server) AddPuller(puller *Puller) {
	server.pullersLock.Lock()
	if _, ok := server.pullers[puller.Path]; !ok {
		server.pullers[puller.Path] = puller
		go puller.Start()
		log.Printf("%v start, now puller size[%d]", puller, len(server.pullers))
	}
	server.pullersLock.Unlock()
}

// RemovePuller - 移除拉流器
func (server *Server) RemovePuller(puller *Puller) {
	server.pullersLock.Lock()
	if _, ok := server.pullers[puller.Path]; ok {
		delete(server.pullers, puller.Path)
		log.Printf("%v end, now puller size[%d]\n", puller, len(server.pullers))
	}
	server.pullersLock.Unlock()
}

// GetPuller - 获取拉流器
func (server *Server) GetPuller(path string) (puller *Puller) {
	server.pullersLock.RLock()
	puller = server.pullers[path]
	server.pullersLock.RUnlock()
	return
}

// GetPullers - 获取所有拉流器
func (server *Server) GetPullers() (pullers map[string]*Puller) {
	pullers = make(map[string]*Puller)
	server.pullersLock.RLock()
	for k, v := range server.pullers {
		pullers[k] = v
	}
	server.pullersLock.RUnlock()
	return
}

// GetPullerSize - 获取拉流器数量
func (server *Server) GetPullerSize() (size int) {
	server.pullersLock.RLock()
	size = len(server.pullers)
	server.pullersLock.RUnlock()
	return
}

// ============================================================================

// AddPusher - 添加推流器
func (server *Server) AddPusher(pusher *Pusher) {
	server.pushersLock.Lock()
	if _, ok := server.pushers[pusher.Path]; !ok {
		server.pushers[pusher.Path] = pusher
		go pusher.Start()
		log.Printf("%v start, now pusher size[%d]", pusher, len(server.pushers))
	}
	server.pushersLock.Unlock()
}

// RemovePusher - 移除推流器
func (server *Server) RemovePusher(pusher *Pusher) {
	server.pushersLock.Lock()
	if _, ok := server.pushers[pusher.Path]; ok {
		delete(server.pushers, pusher.Path)
		log.Printf("%v end, now pusher size[%d]\n", pusher, len(server.pushers))
	}
	server.pushersLock.Unlock()
}

// GetPusher - 获取推流器
func (server *Server) GetPusher(path string) (pusher *Pusher) {
	server.pushersLock.RLock()
	pusher = server.pushers[path]
	server.pushersLock.RUnlock()
	return
}

// GetPushers - 获取所有推流器
func (server *Server) GetPushers() (pushers map[string]*Pusher) {
	pushers = make(map[string]*Pusher)
	server.pushersLock.RLock()
	for k, v := range server.pushers {
		pushers[k] = v
	}
	server.pushersLock.RUnlock()
	return
}

// GetPusherSize - 获取推流器数量
func (server *Server) GetPusherSize() (size int) {
	server.pushersLock.RLock()
	size = len(server.pushers)
	server.pushersLock.RUnlock()
	return
}

// ============================================================================

// GetPlayerByID - 通过会话ID获取播放器
func (server *Server) GetPlayerByID(id string) (player *Player) {
	for k, v := range server.GetPlayers() {
		if k == id {
			player = v
			break
		}
	}
	return
}

// GetPlayerByURL - 通过会话URL获取播放器
func (server *Server) GetPlayerByURL(url string) (player *Player) {
	for _, v := range server.GetPlayers() {
		if v.URL == url {
			player = v
			break
		}
	}
	return
}

// GetPlayers - 获取所有播放器
func (server *Server) GetPlayers() (players map[string]*Player) {
	players = make(map[string]*Player)
	for _, puller := range server.GetPullers() {
		for k, v := range puller.GetPlayers() {
			players[k] = v
		}
	}
	for _, pusher := range server.GetPushers() {
		for k, v := range pusher.GetPlayers() {
			players[k] = v
		}
	}
	return
}

// GetPlayerSize - 获取播放器数量
func (server *Server) GetPlayerSize() (size int) {
	for _, puller := range server.GetPullers() {
		size += len(puller.GetPlayers())
	}
	for _, pusher := range server.GetPushers() {
		size += len(pusher.GetPlayers())
	}
	return
}

// ============================================================================

// AddRecorder - 添加录像器
func (server *Server) AddRecorder(recorder *Recorder) {
	server.recordersLock.Lock()
	if _, ok := server.recorders[recorder.Path]; !ok {
		server.recorders[recorder.Path] = recorder
		go recorder.Start()
		log.Printf("%v start, now recorder size[%d]", recorder, len(server.recorders))
	}
	server.recordersLock.Unlock()
}

// RemoveRecorder - 移除录像器
func (server *Server) RemoveRecorder(recorder *Recorder) {
	server.recordersLock.Lock()
	if _, ok := server.recorders[recorder.Path]; ok {
		delete(server.recorders, recorder.Path)
		log.Printf("%v end, now recorder size[%d]\n", recorder, len(server.recorders))
	}
	server.recordersLock.Unlock()
}

// GetRecorder - 获取录像器
func (server *Server) GetRecorder(path string) (recorder *Recorder) {
	server.recordersLock.RLock()
	recorder = server.recorders[path]
	server.recordersLock.RUnlock()
	return
}

// GetRecorders - 获取所有录像器
func (server *Server) GetRecorders() (recorders map[string]*Recorder) {
	recorders = make(map[string]*Recorder)
	server.recordersLock.RLock()
	for k, v := range server.recorders {
		recorders[k] = v
	}
	server.recordersLock.RUnlock()
	return
}

// GetRecorderSize - 获取录像器数量
func (server *Server) GetRecorderSize() (size int) {
	server.recordersLock.RLock()
	size = len(server.recorders)
	server.recordersLock.RUnlock()
	return
}
