package rtsp

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"
)

// ============================================================================

// Recorder - 录像器数据结构定义
type Recorder struct {
	*Server
	Path    string    //会话路由
	RtspURL string    //RTSP路径
	SaveDir string    //存储路径
	StartAt time.Time //会话开始时候
	Cmd     *exec.Cmd //录像命令
}

// NewRecorder - 新建录像器
func NewRecorder(server *Server, path string) (recorder *Recorder) {
	recorder = &Recorder{
		Server:  server,
		Path:    path,
		StartAt: time.Now(),
	}
	return
}

// ============================================================================

// Start - 启动录像器
func (recorder *Recorder) Start() {
	recorder.RtspURL = fmt.Sprintf("rtsp://127.0.0.1:%d%s", recorder.Server.TCPPort, recorder.Path)
	recorder.SaveDir = path.Join(recorder.Server.SavePath, recorder.Path, time.Now().Format("20060102-150405"))
	if _, err := os.Stat(recorder.SaveDir); os.IsNotExist(err) {
		err = os.MkdirAll(recorder.SaveDir, 0755)
		if err != nil {
			log.Printf("EnsureDir:[%s] err:%v.", recorder.SaveDir, err)
			return
		}
	}
	m3u8path := path.Join(recorder.SaveDir, fmt.Sprintf("out.m3u8"))
	params := []string{"-fflags", "genpts", "-rtsp_transport", "tcp", "-i", recorder.RtspURL, "-hls_time",
		strconv.Itoa(recorder.Server.DurationSec), "-hls_list_size", "0", m3u8path}
	cmd := exec.Command(recorder.Server.FfmpegPath, params...)
	logf, err := os.OpenFile(path.Join(recorder.SaveDir, fmt.Sprintf("recorder.log")), os.O_RDWR|os.O_CREATE, 0755)
	if err == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	err = cmd.Start()
	if err != nil {
		log.Printf("Start ffmpeg err:%v", err)
	}
	recorder.Cmd = cmd
	log.Printf("Add ffmpeg [%v] to pull stream from [%s]", cmd, recorder.Path)
}

// Stop - 停止录像器
func (recorder *Recorder) Stop() {
	player := recorder.Server.GetPlayerByURL(recorder.RtspURL)
	if player != nil {
		player.Teardown()
	}
	recorder.Server.RemoveRecorder(recorder)
	log.Printf("Delete ffmpeg from recorder[%v]", recorder)
}
