package rtsp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teris-io/shortid"
)

// ============================================================================

const (
	// RTSP_VERSION - RTSP版本号
	RTSP_VERSION = "RTSP/1.0"
)

// ============================================================================

// SessionType - 会话类型定义
type SessionType int

const (
	// SESSION_TYPE_UNKNOWN - 未知类型
	SESSION_TYPE_UNKNOWN SessionType = iota
	// SESSION_TYPE_PULLER - 拉流会话
	SESSION_TYPE_PULLER
	// SESSION_TYPE_PUSHER - 推流会话
	SESSION_TYPE_PUSHER
	// SESSION_TYPE_PLAYER - 播放会话
	SESSION_TYPE_PLAYER
)

func (st SessionType) String() string {
	switch st {
	case SESSION_TYPE_UNKNOWN:
		return "unknown"
	case SESSION_TYPE_PULLER:
		return "puller"
	case SESSION_TYPE_PUSHER:
		return "pusher"
	case SESSION_TYPE_PLAYER:
		return "player"
	}
	return "unknow"
}

// TransType - 传输类型定义
type TransType int

const (
	// TRANS_TYPE_TCP - TCP传输
	TRANS_TYPE_TCP TransType = iota
	// TRANS_TYPE_UDP - UDP传输
	TRANS_TYPE_UDP
)

func (tt TransType) String() string {
	switch tt {
	case TRANS_TYPE_TCP:
		return "TCP"
	case TRANS_TYPE_UDP:
		return "UDP"
	}
	return "unknow"
}

// RTPType - RTP类型定义
type RTPType int

const (
	// RTP_TYPE_AUDIO - 音频数据
	RTP_TYPE_AUDIO RTPType = iota
	// RTP_TYPE_VIDEO - 视频数据
	RTP_TYPE_VIDEO
	// RTP_TYPE_AUDIOCONTROL - 音频控制
	RTP_TYPE_AUDIOCONTROL
	// RTP_TYPE_VIDEOCONTROL - 视频控制
	RTP_TYPE_VIDEOCONTROL
)

func (rt RTPType) String() string {
	switch rt {
	case RTP_TYPE_AUDIO:
		return "audio"
	case RTP_TYPE_VIDEO:
		return "video"
	case RTP_TYPE_AUDIOCONTROL:
		return "audio control"
	case RTP_TYPE_VIDEOCONTROL:
		return "video control"
	}
	return "unknow"
}

// RTPPack - RTP包数据
type RTPPack struct {
	Type   RTPType
	Buffer *bytes.Buffer
}

// ============================================================================

// Session - 会话数据结构定义
type Session struct {
	// 基本信息
	SessionID string            // 会话ID
	Stoped    bool              // 停止标记
	Online    bool              // 在线标记
	Server    *Server           // 关联RTSP服务
	Conn      *net.TCPConn      // 关联TCP连接
	connRW    *bufio.ReadWriter // 连接读写缓存
	connWLock sync.RWMutex      // 连接读写锁

	// 传输信息
	Type         SessionType         // 会话类型
	TransType    TransType           // 传输类型
	Addr         string              // 源地址
	Path         string              // 会话路由
	URL          string              // 会话URL
	REQ          *Request            // 主动请求数据
	Userinfo     *url.Userinfo       // RTSP用户信息
	Authenticate map[string]string   // RTSP认证凭据
	SDPRaw       string              // SDP元数据
	SDPMap       map[string]*SDPInfo // SDP解析数据

	// RTSP音视频信息
	AControl string // 音频控制地址
	VControl string // 视频控制地址
	ACodec   string // 音频编码协议
	VCodec   string // 视频编码协议

	// 会话统计信息
	InBytes  int       // 输入流量
	OutBytes int       // 输出流量
	StartAt  time.Time // 会话开始时候
	Timeout  int       // 会话超时时间
	CSeq     int       // 会话序号

	// RTP通道信息
	aRTPChannel        int // 音频RTP数据通道
	aRTPControlChannel int // 音频RTP控制通道
	vRTPChannel        int // 视频RTP数据通道
	vRTPControlChannel int // 视频RTP控制通道

	// 关联处理模块
	Puller      *Puller          // 拉流器
	Pusher      *Pusher          // 推流器
	Player      *Player          // 播放器
	UDPClient   *UDPClient       // UDP客户端
	RTPHandles  []func(*RTPPack) // RTP数据处理函数
	StopHandles []func()         // 会话停止处理函数
}

// ============================================================================

// String - 会话字符串化
func (session *Session) String() string {
	return fmt.Sprintf("session[%v][%v][%s][%s]", session.Type, session.TransType, session.Path, session.SessionID)
}

// NewSession - 新建会话
func NewSession(server *Server, conn *net.TCPConn, stype SessionType, name, addr string) *Session {
	// 解析RTSP地址
	var rstpURL string
	var userInfo *url.Userinfo
	if name != "" && addr != "" {
		url, err := url.Parse(addr)
		if err != nil {
			log.Println(err)
			return nil
		}
		rstpURL = url.Scheme + "://" + url.Host + url.Path
		userInfo = url.User
	}
	// 新建RTSP会话
	session := &Session{
		SessionID:   shortid.MustGenerate(),
		Server:      server,
		Conn:        conn,
		connRW:      bufio.NewReadWriter(bufio.NewReaderSize(conn, Instance.NetBufSize), bufio.NewWriterSize(conn, Instance.NetBufSize)),
		Type:        stype,
		Addr:        addr,
		Path:        "/" + name,
		URL:         rstpURL,
		Userinfo:    userInfo,
		StartAt:     time.Now(),
		Timeout:     Instance.NetTimeout,
		RTPHandles:  make([]func(*RTPPack), 0),
		StopHandles: make([]func(), 0),
	}
	return session
}

// ============================================================================

// Start - 启动会话
func (session *Session) Start() {
	// 结束处理
	defer func() {
		if session.Type == SESSION_TYPE_PULLER {
			session.Reconnect()
			go session.Start()
		} else {
			session.Stop()
		}
	}()
	// 监听接收数据
	buf1 := make([]byte, 1)
	buf2 := make([]byte, 2)
	for !session.Stoped {
		// 连接处理
		if _, err := io.ReadFull(session.connRW, buf1); err != nil {
			log.Println(session, err)
			return
		}
		session.Online = true
		session.Conn.SetDeadline(time.Now().Add(time.Duration(session.Timeout) * time.Second))
		// 协议处理
		if buf1[0] == 0x24 { // RTP协议
			if _, err := io.ReadFull(session.connRW, buf1); err != nil {
				log.Println(err)
				return
			}
			if _, err := io.ReadFull(session.connRW, buf2); err != nil {
				log.Println(err)
				return
			}
			channel := int(buf1[0])
			rtpLen := int(binary.BigEndian.Uint16(buf2))
			rtpBytes := make([]byte, rtpLen)
			if _, err := io.ReadFull(session.connRW, rtpBytes); err != nil {
				log.Println(err)
				return
			}
			rtpBuf := bytes.NewBuffer(rtpBytes)
			var pack *RTPPack
			switch channel {
			case session.aRTPChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_AUDIO,
					Buffer: rtpBuf,
				}
			case session.aRTPControlChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_AUDIOCONTROL,
					Buffer: rtpBuf,
				}
			case session.vRTPChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_VIDEO,
					Buffer: rtpBuf,
				}
			case session.vRTPControlChannel:
				pack = &RTPPack{
					Type:   RTP_TYPE_VIDEOCONTROL,
					Buffer: rtpBuf,
				}
			default:
				log.Printf("unknow rtp pack type, %d", channel)
				continue
			}
			if pack == nil {
				log.Printf("session tcp got nil rtp pack")
				continue
			}
			session.InBytes += rtpLen + 4
			for _, h := range session.RTPHandles {
				h(pack)
			}
		} else { // RTSP协议
			rcvBuf := bytes.NewBuffer(nil)
			rcvBuf.Write(buf1)
			for !session.Stoped {
				line, isPrefix, err := session.connRW.ReadLine()
				if err != nil {
					log.Println(err)
					return
				}
				rcvBuf.Write(line)
				if !isPrefix {
					rcvBuf.WriteString("\r\n")
				}
				if len(line) == 0 {
					// 拉流会话
					if session.Type == SESSION_TYPE_PULLER {
						res := ParseResponse(rcvBuf.String())
						if res == nil {
							break
						}
						contentLen := res.GetContentLength()
						session.InBytes += contentLen
						if contentLen > 0 {
							bodyBuf := make([]byte, contentLen)
							if n, err := io.ReadFull(session.connRW, bodyBuf); err != nil {
								log.Println(err)
								return
							} else if n != contentLen {
								log.Printf("read rtsp response body failed, expect size[%d], got size[%d]", contentLen, n)
								return
							}
							res.Body = string(bodyBuf)
						}
						session.handleResponse(res)
						break
					}
					// 推流、播放会话
					if session.Type != SESSION_TYPE_PULLER {
						req := ParseRequest(rcvBuf.String())
						if req == nil {
							break
						}
						session.InBytes += rcvBuf.Len()
						contentLen := req.GetContentLength()
						session.InBytes += contentLen
						if contentLen > 0 {
							bodyBuf := make([]byte, contentLen)
							if n, err := io.ReadFull(session.connRW, bodyBuf); err != nil {
								log.Println(err)
								return
							} else if n != contentLen {
								log.Printf("read rtsp request body failed, expect size[%d], got size[%d]", contentLen, n)
								return
							}
							req.Body = string(bodyBuf)
						}
						session.handleRequest(req)
						break
					}
				}
			}
		}
	}
}

// Stop - 停止会话
func (session *Session) Stop() {
	if session.Stoped {
		return
	}
	session.Stoped = true
	session.Online = false
	session.Authenticate = nil
	for _, h := range session.StopHandles {
		h()
	}
	if session.Conn != nil {
		session.connRW.Flush()
		session.Conn.Close()
		session.Conn = nil
	}
	if session.UDPClient != nil {
		session.UDPClient.Stop()
		session.UDPClient = nil
	}
}

// Trigger - 主动触发会话
func (session *Session) Trigger() {
	session.CSeq = 1
	session.Type = SESSION_TYPE_PULLER
	req := NewRequest(OPTIONS, session.URL, strconv.Itoa(session.CSeq), session.SessionID, "")
	log.Println("\n========================RTSP TRIGGER========================")
	log.Printf("\n<<<<<<----------------------------------------------------\n[%s : %s] %v", req.Method, session.SessionID, req)
	outBytes := []byte(req.String())
	session.Conn.SetDeadline(time.Now().Add(time.Duration(session.Timeout) * time.Second))
	session.connWLock.Lock()
	session.connRW.Write(outBytes)
	session.connRW.Flush()
	session.connWLock.Unlock()
	session.OutBytes += len(outBytes)
	session.REQ = req
}

// Teardown - 主动结束会话
func (session *Session) Teardown() {
	session.CSeq++
	session.Type = SESSION_TYPE_PULLER
	req := NewRequest(TEARDOWN, session.URL, strconv.Itoa(session.CSeq), session.SessionID, "")
	req.SetAuthorization(session.URL, session.Userinfo, session.Authenticate)
	log.Println("\n========================RTSP TEARDOWN=======================")
	log.Printf("\n<<<<<<----------------------------------------------------\n[%s : %s] %v", req.Method, session.SessionID, req)
	outBytes := []byte(req.String())
	session.connWLock.Lock()
	session.connRW.Write(outBytes)
	session.connRW.Flush()
	session.connWLock.Unlock()
	session.OutBytes += len(outBytes)
	session.Stop()
}

// Reconnect - 断线重连
func (session *Session) Reconnect() {
	if session.Stoped {
		return
	}
	session.Online = false
	log.Println("\n==========================Reconnect=========================")
	// 断开连接数据
	session.Authenticate = nil
	if session.Conn != nil {
		session.connRW.Flush()
		session.Conn.Close()
		session.Conn = nil
	}
	if session.UDPClient != nil {
		session.UDPClient.Stop()
		session.UDPClient = nil
	}
	// 建立TCP连接
	url, err := url.Parse(session.URL)
	if err != nil {
		log.Println(err)
		session.Stop()
		return
	}
	ipaddr, err := net.ResolveTCPAddr("tcp", url.Host)
	if err != nil {
		log.Println(err)
		session.Stop()
		return
	}
	conn, err := net.DialTCP("tcp", nil, ipaddr)
	if err != nil {
		log.Println(err)
		session.Stop()
		return
	}
	// 设置连接参数
	if err := conn.SetReadBuffer(Instance.NetBufSize); err != nil {
		log.Println(err)
		session.Stop()
		return
	}
	if err := conn.SetWriteBuffer(Instance.NetBufSize); err != nil {
		log.Println(err)
		session.Stop()
		return
	}
	session.SessionID = shortid.MustGenerate()
	session.Conn = conn
	session.connRW = bufio.NewReadWriter(bufio.NewReaderSize(conn, Instance.NetBufSize), bufio.NewWriterSize(conn, Instance.NetBufSize))
	session.Trigger()
}

// ============================================================================

// handleResponse - 回应数据处理(拉流)
func (session *Session) handleResponse(res *Response) {
	log.Printf("\n>>>>>>----------------------------------------------------\n[%s : %s] %v", session.SessionID, session.REQ.Method, res)
	ses := res.GetSessionInfo()
	if id, ok := ses["id"]; ok {
		session.SessionID = id
	}
	// 请求数据
	session.CSeq++
	req := NewRequest("", session.URL, strconv.Itoa(session.CSeq), session.SessionID, "")
	defer func() {
		if p := recover(); p != nil {
			log.Println("[PANIC] ", p)
		}
		if req == nil {
			session.CSeq--
			return
		}
		req.SetAuthorization(session.URL, session.Userinfo, session.Authenticate)
		// 发送后续请求
		log.Printf("\n<<<<<<----------------------------------------------------\n[%s : %s] %v", session.SessionID, req.Method, req)
		outBytes := []byte(req.String())
		session.connWLock.Lock()
		session.connRW.Write(outBytes)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(outBytes)
		session.REQ = req
		// 后期处理
		switch session.REQ.Method {
		case "PLAY", "RECORD":
			switch session.Type {
			case SESSION_TYPE_PULLER:
				session.Server.AddPuller(session.Puller)
			case SESSION_TYPE_PUSHER:
			case SESSION_TYPE_PLAYER:
			}
		}
	}()
	// 回应处理
	switch session.REQ.Method {
	case "OPTIONS": // OPTIONS请求回应
		if res.StatusCode == 200 {
			// 后续请求：DESCRIBE
			req.Method = DESCRIBE
			req.Header["Accept"] = "application/sdp"
		} else {
			req = nil
			log.Printf("Method[%s] response error [%d]%s\n", session.REQ.Method, res.StatusCode, res.Status)
			session.Reconnect()
		}
	case "DESCRIBE": // DESCRIBE请求回应
		if res.StatusCode == 401 {
			if session.Authenticate != nil {
				// 未通过验证：终止通话
				req.Method = TEARDOWN
			} else {
				// 后续请求：DESCRIBE(带认证信息)
				req.Method = DESCRIBE
				session.Authenticate = res.GetAuthenticate()
				if session.Authenticate == nil {
					// 获取凭证失败：终止通话
					req.Method = TEARDOWN
				}
			}
		} else if res.StatusCode == 200 {
			// SDP数据解析
			session.SDPRaw = res.Body
			session.SDPMap = ParseSDP(res.Body)
			sdp, ok := session.SDPMap["audio"]
			if ok {
				session.AControl = sdp.Control
				session.ACodec = sdp.Codec
				log.Printf("audio codec[%s]\n", session.ACodec)
			}
			sdp, ok = session.SDPMap["video"]
			if ok {
				session.VControl = sdp.Control
				session.VCodec = sdp.Codec
				log.Printf("video codec[%s]\n", session.VCodec)
			}
			// 新建拉流器
			if session.Server.GetPuller(session.Path) == nil && session.Server.GetPusher(session.Path) == nil {
				session.Puller = NewPuller(session)
			}
			// 后续请求：SETUP(视频)
			req.Method = SETUP
			req.URL = session.VControl
			req.Header["Transport"] = "RTP/AVP/TCP;unicast;interleaved=0-1;"
			session.TransType = TRANS_TYPE_TCP
			session.vRTPChannel = 0
			session.vRTPControlChannel = 1
		} else {
			req = nil
			log.Printf("Method[%s] response error [%d]%s\n", session.REQ.Method, res.StatusCode, res.Status)
			session.Reconnect()
		}
	case "SETUP": // SETUP请求回应
		if res.StatusCode == 200 {
			if session.REQ.URL == session.VControl && session.AControl != "" {
				// 后续请求：SETUP(音频)
				ses := res.GetSessionInfo()
				if timeout, ok := ses["timeout"]; ok {
					if timeval, err := strconv.Atoi(timeout); err == nil {
						session.Timeout = timeval
					}
				}
				req.Method = SETUP
				req.URL = session.AControl
				req.Header["Session"] = session.SessionID
				req.Header["Transport"] = "RTP/AVP/TCP;unicast;interleaved=2-3;"
				session.aRTPChannel = 2
				session.aRTPControlChannel = 3
			} else {
				// 后续请求：PLAY
				req.Method = PLAY
				req.Header["Range"] = "npt=0.000-"
			}
		} else {
			req = nil
			log.Printf("Method[%s] response error [%d]%s\n", session.REQ.Method, res.StatusCode, res.Status)
			session.Reconnect()
		}
	case "PLAY": // PLAY请求回应
		req = nil
		if res.StatusCode != 200 {
			log.Printf("Method[%s] response error [%d]%s\n", session.REQ.Method, res.StatusCode, res.Status)
			session.Reconnect()
		}
	case "RECORD": // RECORD请求回应
		req = nil
	case "TEARDOWN": // TEARDOWN请求回应
		req = nil
		session.Stop()
	default:
		req = nil
	}
}

// handleRequest - 请求数据处理(推流、播放)
func (session *Session) handleRequest(req *Request) {
	if session.REQ == nil && req.Method == OPTIONS {
		log.Println("\n========================RTSP STATRT=======================")
	}
	ses := req.GetSessionInfo()
	if id, ok := ses["id"]; ok {
		session.SessionID = id
	}
	// 回应数据
	session.REQ = req
	log.Printf("\n>>>>>>----------------------------------------------------\n[%s : %s] %v", session.SessionID, req.Method, req)
	res := NewResponse(200, "OK", req.Header["CSeq"], session.SessionID, "")
	defer func() {
		if p := recover(); p != nil {
			log.Println("[PANIC] ", p)
			res.StatusCode = 500
			res.Status = fmt.Sprintf("Inner Server Error, %v", p)
		}
		if res == nil {
			return
		}
		// 发送回应数据
		log.Printf("\n<<<<<<----------------------------------------------------\n[%s : %s] %v", session.SessionID, req.Method, res)
		outBytes := []byte(res.String())
		session.connWLock.Lock()
		session.connRW.Write(outBytes)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(outBytes)
		// 后期处理
		switch req.Method {
		case "PLAY", "RECORD":
			switch session.Type {
			case SESSION_TYPE_PULLER:
			case SESSION_TYPE_PUSHER:
				session.Server.AddPusher(session.Pusher)
			case SESSION_TYPE_PLAYER:
				if session.Puller != nil {
					session.Puller.AddPlayer(session.Player)
				} else if session.Pusher != nil {
					session.Pusher.AddPlayer(session.Player)
				}
			}
		case "TEARDOWN":
			session.Stop()
		}
	}()
	// 请求处理
	switch req.Method {
	case "OPTIONS": // OPTIONS请求处理
		res.Header["Public"] = "DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE, OPTIONS, ANNOUNCE, RECORD"
	case "ANNOUNCE": // ANNOUNCE请求处理(推流)
		// 地址解析
		session.Type = SESSION_TYPE_PUSHER
		session.Addr = req.URL
		session.URL = req.URL
		url, err := url.Parse(req.URL)
		if err != nil {
			res.StatusCode = 500
			res.Status = "Invalid URL"
			return
		}
		session.Path = url.Path
		// SDP解析
		session.SDPRaw = req.Body
		session.SDPMap = ParseSDP(req.Body)
		sdp, ok := session.SDPMap["audio"]
		if ok {
			session.AControl = sdp.Control
			session.ACodec = sdp.Codec
			log.Printf("audio codec[%s]\n", session.ACodec)
		}
		sdp, ok = session.SDPMap["video"]
		if ok {
			session.VControl = sdp.Control
			session.VCodec = sdp.Codec
			log.Printf("video codec[%s]\n", session.VCodec)
		}
		if session.Server.GetPuller(session.Path) == nil && session.Server.GetPusher(session.Path) == nil {
			session.Pusher = NewPusher(session)
		} else {
			res.StatusCode = 406
			res.Status = "Not Acceptable"
			return
		}
	case "DESCRIBE": // DESCRIBE请求处理(播放)
		// 地址解析
		session.Type = SESSION_TYPE_PLAYER
		session.Addr = req.URL
		session.URL = req.URL
		url, err := url.Parse(req.URL)
		if err != nil {
			res.StatusCode = 500
			res.Status = "Invalid URL"
			return
		}
		session.Path = url.Path
		// 查找拉流器
		puller := session.Server.GetPuller(session.Path)
		if puller != nil {
			session.Player = NewPlayer(session, puller, nil)
			session.Puller = puller
			session.AControl = puller.AControl
			session.VControl = puller.VControl
			session.ACodec = puller.ACodec
			session.VCodec = puller.VCodec
			res.SetBody(ConvertSDP(puller.SDPRaw, req.URL))
		}
		// 查找推流器
		pusher := session.Server.GetPusher(session.Path)
		if pusher != nil {
			session.Player = NewPlayer(session, nil, pusher)
			session.Pusher = pusher
			session.AControl = pusher.AControl
			session.VControl = pusher.VControl
			session.ACodec = pusher.ACodec
			session.VCodec = pusher.VCodec
			res.SetBody(ConvertSDP(pusher.SDPRaw, req.URL))
		}
		// 未找到资源
		if puller == nil && pusher == nil {
			res.StatusCode = 404
			res.Status = "NOT FOUND"
		}
	case "SETUP": // SETUP请求处理
		ts := req.Header["Transport"]
		control := req.URL[strings.LastIndex(req.URL, "/")+1:]
		// TCP控制通道
		mtcp := regexp.MustCompile("interleaved=(\\d+)(-(\\d+))?")
		if tcpMatchs := mtcp.FindStringSubmatch(ts); tcpMatchs != nil {
			session.TransType = TRANS_TYPE_TCP
			if control == GetCtl(session.AControl) {
				session.aRTPChannel, _ = strconv.Atoi(tcpMatchs[1])
				session.aRTPControlChannel, _ = strconv.Atoi(tcpMatchs[3])
			} else if control == GetCtl(session.VControl) {
				session.vRTPChannel, _ = strconv.Atoi(tcpMatchs[1])
				session.vRTPControlChannel, _ = strconv.Atoi(tcpMatchs[3])
			}
		}
		// UCP控制通道
		mudp := regexp.MustCompile("client_port=(\\d+)(-(\\d+))?")
		if udpMatchs := mudp.FindStringSubmatch(ts); udpMatchs != nil {
			session.TransType = TRANS_TYPE_UDP
			// 新建UDP客户端及服务端
			if session.UDPClient == nil {
				session.UDPClient = &UDPClient{
					Session: session,
				}
			}
			if session.Type == SESSION_TYPE_PUSHER && session.Pusher.UDPServer == nil {
				session.Pusher.UDPServer = &UDPServer{
					Session: session,
				}
			}
			// 音频UDP通道
			if control == GetCtl(session.AControl) {
				session.UDPClient.APort, _ = strconv.Atoi(udpMatchs[1])
				session.UDPClient.AControlPort, _ = strconv.Atoi(udpMatchs[3])
				if err := session.UDPClient.SetupAudio(); err != nil {
					res.StatusCode = 500
					res.Status = fmt.Sprintf("udp client setup audio error, %v", err)
					return
				}
				if session.Type == SESSION_TYPE_PUSHER {
					if err := session.Pusher.UDPServer.SetupAudio(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp server setup audio error, %v", err)
						return
					}
					tss := strings.Split(ts, ";")
					idx := -1
					for i, val := range tss {
						if val == udpMatchs[0] {
							idx = i
						}
					}
					tail := append([]string{}, tss[idx+1:]...)
					tss = append(tss[:idx+1], fmt.Sprintf("server_port=%d-%d", session.Pusher.UDPServer.APort, session.Pusher.UDPServer.AControlPort))
					tss = append(tss, tail...)
					ts = strings.Join(tss, ";")
				}
			}
			// 视频UDP通道
			if control == GetCtl(session.VControl) {
				session.UDPClient.VPort, _ = strconv.Atoi(udpMatchs[1])
				session.UDPClient.VControlPort, _ = strconv.Atoi(udpMatchs[3])
				if err := session.UDPClient.SetupVideo(); err != nil {
					res.StatusCode = 500
					res.Status = fmt.Sprintf("udp client setup video error, %v", err)
					return
				}
				if session.Type == SESSION_TYPE_PUSHER {
					if err := session.Pusher.UDPServer.SetupVideo(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp server setup video error, %v", err)
						return
					}
					tss := strings.Split(ts, ";")
					idx := -1
					for i, val := range tss {
						if val == udpMatchs[0] {
							idx = i
						}
					}
					tail := append([]string{}, tss[idx+1:]...)
					tss = append(tss[:idx+1], fmt.Sprintf("server_port=%d-%d", session.Pusher.UDPServer.VPort, session.Pusher.UDPServer.VControlPort))
					tss = append(tss, tail...)
					ts = strings.Join(tss, ";")
				}
			}
		}
		res.Header["Transport"] = ts
	case "PLAY": // PLAY请求处理
		res.Header["Range"] = req.Header["Range"]
	case "RECORD": // RECORD请求处理
	}
}
