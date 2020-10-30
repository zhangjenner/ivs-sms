package rtsp

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================================

const (
	// DESCRIBE - Client to server for presentation and stream objects; recommended
	DESCRIBE = "DESCRIBE"
	// ANNOUNCE - Bidirectional for client and stream objects; optional
	ANNOUNCE = "ANNOUNCE"
	// GET_PARAMETER - Bidirectional for client and stream objects; optional
	GET_PARAMETER = "GET_PARAMETER"
	// OPTIONS - Bidirectional for client and stream objects; required for Client to server, optional for server to client
	OPTIONS = "OPTIONS"
	// PAUSE - Client to server for presentation and stream objects; recommended
	PAUSE = "PAUSE"
	// PLAY - Client to server for presentation and stream objects; required
	PLAY = "PLAY"
	// RECORD - Client to server for presentation and stream objects; optional
	RECORD = "RECORD"
	// REDIRECT - Server to client for presentation and stream objects; optional
	REDIRECT = "REDIRECT"
	// SETUP - Client to server for stream objects; required
	SETUP = "SETUP"
	// SET_PARAMETER - Bidirectional for presentation and stream objects; optional
	SET_PARAMETER = "SET_PARAMETER"
	// TEARDOWN - Client to server for presentation and stream objects; required
	TEARDOWN = "TEARDOWN"
	// DATA - Server to client for presentation and stream objects; optional
	DATA = "DATA"
)

// ============================================================================

// Request - RTSP请求数据结构定义
type Request struct {
	Method  string
	URL     string
	Version string
	Header  map[string]string
	Content string
	Body    string
}

// ============================================================================

// NewRequest - 新建RTSP请求数据
func NewRequest(method, url, cSeq, sid, body string) *Request {
	req := &Request{
		Method:  method,
		URL:     url,
		Version: RTSP_VERSION,
		Header:  map[string]string{"CSeq": cSeq, "Session": sid, "User-Agent": "MLBVC"},
		Body:    body,
	}
	len := len(body)
	if len > 0 {
		req.Header["Content-Length"] = strconv.Itoa(len)
	} else {
		delete(req.Header, "Content-Length")
	}
	return req
}

// SetAuthorization - 设置RTSP请求摘要认证
func (r *Request) SetAuthorization(uri string, user *url.Userinfo, authenticate map[string]string) {
	if password, ok := user.Password(); ok && authenticate != nil {
		userMd5 := MD5(user.Username() + ":" + authenticate["realm"] + ":" + password)
		urlMd5 := MD5(r.Method + ":" + uri)
		response := MD5(userMd5 + ":" + authenticate["nonce"] + ":" + urlMd5)
		r.Header["Authorization"] = fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"",
			user.Username(), authenticate["realm"], authenticate["nonce"], uri, response)
	}
}

// String - 字符串化RTSP请求数据
func (r *Request) String() string {
	str := fmt.Sprintf("%s %s %s\r\n", r.Method, r.URL, r.Version)
	for key, value := range r.Header {
		str += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	str += "\r\n"
	str += r.Body
	return str
}

// ============================================================================

// ParseRequest - 解析RTSP请求数据
func ParseRequest(content string) *Request {
	lines := strings.Split(strings.TrimSpace(content), "\r\n")
	if len(lines) == 0 {
		return nil
	}

	items := regexp.MustCompile("\\s+").Split(strings.TrimSpace(lines[0]), -1)
	if len(items) < 3 {
		return nil
	}
	if !strings.HasPrefix(items[2], "RTSP") {
		log.Printf("invalid rtsp request, line[0] %s", lines[0])
		return nil
	}
	header := make(map[string]string)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		headerItems := regexp.MustCompile(":\\s+").Split(line, 2)
		if len(headerItems) < 2 {
			continue
		}
		if _, ok := header[headerItems[0]]; !ok {
			header[headerItems[0]] = headerItems[1]
		} else {
			header[headerItems[0]+"-1"] = headerItems[1]
		}
	}

	return &Request{
		Method:  items[0],
		URL:     items[1],
		Version: items[2],
		Header:  header,
		Content: content,
		Body:    "",
	}
}

// GetContentLength - 获取RTSP请求数据负载长度
func (r *Request) GetContentLength() int {
	if data, ok := r.Header["Content-Length"]; ok {
		if v, err := strconv.ParseInt(data, 10, 64); err == nil {
			return int(v)
		}
	}
	return 0
}

// GetSessionInfo -  获取RTSP回应会话信息
func (r *Request) GetSessionInfo() map[string]string {
	session := make(map[string]string)
	if ses, ok := r.Header["Session"]; ok {
		items := regexp.MustCompile(";").Split(strings.TrimSpace(ses), -1)
		session["id"] = items[0]
		for i := 1; i < len(items); i++ {
			item := strings.TrimSpace(items[i])
			elems := regexp.MustCompile("=").Split(item, 2)
			session[elems[0]] = elems[1]
		}
	}
	return session
}

// MD5 - MD5编码
func MD5(str string) string {
	encoder := md5.New()
	encoder.Write([]byte(str))
	return hex.EncodeToString(encoder.Sum(nil))
}
