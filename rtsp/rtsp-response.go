package rtsp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================================

// Response - RTSP回应数据结构定义
type Response struct {
	Version    string
	StatusCode int
	Status     string
	Header     map[string]string
	Body       string
}

// ============================================================================

// NewResponse - 新建RTSP回应数据
func NewResponse(statusCode int, status, cSeq, sid, body string) *Response {
	res := &Response{
		Version:    RTSP_VERSION,
		StatusCode: statusCode,
		Status:     status,
		Header:     map[string]string{"CSeq": cSeq, "Session": sid},
		Body:       body,
	}
	len := len(body)
	if len > 0 {
		res.Header["Content-Length"] = strconv.Itoa(len)
	} else {
		delete(res.Header, "Content-Length")
	}
	return res
}

// SetBody - 设置RTSP回应负载数据
func (r *Response) SetBody(body string) {
	len := len(body)
	r.Body = body
	if len > 0 {
		r.Header["Content-Length"] = strconv.Itoa(len)
	} else {
		delete(r.Header, "Content-Length")
	}
}

// String - 字符串化RTSP回应数据
func (r *Response) String() string {
	str := fmt.Sprintf("%s %d %s\r\n", r.Version, r.StatusCode, r.Status)
	for key, value := range r.Header {
		str += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	str += "\r\n"
	str += r.Body
	return str
}

// ============================================================================

// ParseResponse - 解析RTSP回应数据
func ParseResponse(content string) *Response {
	lines := strings.Split(strings.TrimSpace(content), "\r\n")
	if len(lines) == 0 {
		return nil
	}

	items := regexp.MustCompile("\\s+").Split(strings.TrimSpace(lines[0]), -1)
	if len(items) < 3 {
		return nil
	}
	version := items[0]
	statusCode, _ := strconv.Atoi(items[1])
	status := items[2]

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

	return &Response{
		Version:    version,
		StatusCode: statusCode,
		Status:     status,
		Header:     header,
		Body:       "",
	}
}

// GetContentLength - 获取RTSP回应数据负载长度
func (r *Response) GetContentLength() int {
	if len, ok := r.Header["Content-Length"]; ok {
		if v, err := strconv.ParseInt(len, 10, 64); err == nil {
			return int(v)
		}
	}
	return 0
}

// GetAuthenticate - 获取RTSP回应认证凭据
func (r *Response) GetAuthenticate() map[string]string {
	authenticate := make(map[string]string)
	for _, tag := range []string{"WWW-Authenticate", "WWW-Authenticate-1"} {
		if auth, ok := r.Header[tag]; ok {
			items := regexp.MustCompile(",\\s+").Split(strings.TrimSpace(auth[6:]), -1)
			for _, item := range items {
				item := strings.TrimSpace(item)
				elems := regexp.MustCompile("=").Split(item, 2)
				authenticate[elems[0]] = elems[1][1 : len(elems[1])-1]
			}
			_, ok1 := authenticate["realm"]
			_, ok2 := authenticate["nonce"]
			if ok1 && ok2 {
				return authenticate
			}
		}
	}
	return nil
}

// GetSessionInfo -  获取RTSP回应会话信息
func (r *Response) GetSessionInfo() map[string]string {
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
