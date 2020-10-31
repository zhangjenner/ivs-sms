package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"ivs-sms/rtsp"
	"ivs-sms/utils"

	"github.com/kardianos/service"
)

//=============================================================================

// program - 服务程序定义
type program struct {
	rtspPort   int
	rtspServer *rtsp.Server
	httpPort   int
	httpServer *http.Server
}

//-----------------------------------------------------------------------------

// Start - 启动服务
func (prog *program) Start(s service.Service) (err error) {
	log.Println("********** START **********")
	if utils.IsPortInUse(prog.httpPort) {
		err = fmt.Errorf("HTTP port[%d] In Use", prog.httpPort)
		return
	}
	if utils.IsPortInUse(prog.rtspPort) {
		err = fmt.Errorf("PUSH port[%d] In Use", prog.rtspPort)
		return
	}
	prog.StartRTSP()
	prog.StartHTTP()
	return
}

// Stop - 停止服务
func (prog *program) Stop(s service.Service) (err error) {
	defer log.Println("********** STOP **********")
	defer utils.CloseLogger()
	prog.StopHTTP()
	prog.StopRTSP()
	return
}

//-----------------------------------------------------------------------------

//StartRTSP - 启动RTSP服务
func (prog *program) StartRTSP() (err error) {
	// 创建服务
	prog.rtspServer = rtsp.GetServer()
	prog.rtspPort = utils.Conf.Section("rtsp").Key("port").MustInt(10003)
	cfg := &rtsp.ServerCfg{
		TCPPort:     prog.rtspPort,
		NetBufSize:  utils.Conf.Section("rtsp").Key("network_timeout").MustInt(60),
		NetTimeout:  utils.Conf.Section("rtsp").Key("network_buffer").MustInt(1048576),
		FfmpegPath:  utils.Conf.Section("record").Key("ffmpeg_path").MustString(""),
		SavePath:    utils.Conf.Section("record").Key("save_path").MustString(""),
		DurationSec: utils.Conf.Section("record").Key("duration_sec").MustInt(1800),
	}
	if err = prog.rtspServer.CfgServer(cfg); err != nil {
		log.Println(err)
		log.Println("Start RTSP server error", err)
		return
	}
	// 启动服务
	addr := fmt.Sprintf("rtsp://%s:%d", utils.LocalIP(), prog.rtspPort)
	log.Println("RTSP server start -->", addr)
	go func() {
		if err := prog.rtspServer.Start(); err != nil {
			log.Println("Start RTSP server error", err)
		}
		log.Println("RTSP server end")
	}()
	return
}

// StopRTSP - 停止RTSP服务
func (prog *program) StopRTSP() (err error) {
	if prog.rtspServer == nil {
		err = fmt.Errorf("RTSP server not found")
		return
	}
	prog.rtspServer.Stop()
	return
}

//-----------------------------------------------------------------------------

// StartHTTP - 启动HTTP服务
func (prog *program) StartHTTP() (err error) {
	// prog.httpPort = utils.Conf.Section("http").Key("port").MustInt(10002)
	// prog.httpServer = &http.Server{
	// 	Addr:              fmt.Sprintf(":%d", prog.httpPort),
	// 	Handler:           routers.Router,
	// 	ReadHeaderTimeout: 5 * time.Second,
	// }
	// addr := fmt.Sprintf("http://%s:%d", utils.LocalIP(), prog.httpPort)
	// log.Println("HTTP server start -->", addr)
	// go func() {
	// 	if err := prog.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	// 		log.Println("Start HTTP server error", err)
	// 	}
	// 	log.Println("HTTP server end")
	// }()
	return
}

// StopHTTP - 停止HTTP服务
func (prog *program) StopHTTP() (err error) {
	// if prog.httpServer == nil {
	// 	err = fmt.Errorf("HTTP server not found")
	// 	return
	// }
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// if err = prog.httpServer.Shutdown(ctx); err != nil {
	// 	return
	// }
	return
}

//=============================================================================

// 主程序
func main() {
	// 日志配置
	utils.LoadConf()
	log.SetPrefix("[IVS-SMS] ")
	log.SetFlags(log.LstdFlags)
	if utils.Debug {
		log.SetFlags(log.Lshortfile | log.LstdFlags)
		log.SetOutput(utils.GetLogger())
	}

	// 新建服务
	sec := utils.Conf.Section("service")
	svcConfig := &service.Config{
		Name:        sec.Key("name").MustString(""),
		DisplayName: sec.Key("display_name").MustString(""),
		Description: sec.Key("description").MustString(""),
	}
	var srv, err = service.New(new(program), svcConfig)
	if err != nil {
		log.Println(err)
		utils.PauseExit()
	}

	// 操作服务
	if len(os.Args) > 1 {
		if os.Args[1] == "install" || os.Args[1] == "stop" {
			log.Println(svcConfig.Name, os.Args[1], "...")
		}
		if err = service.Control(srv, os.Args[1]); err != nil {
			log.Println(err)
			utils.PauseExit()
		}
		log.Println(svcConfig.Name, os.Args[1], "ok")
		return
	}

	// 运行服务
	if err = srv.Run(); err != nil {
		log.Println(err)
		utils.PauseExit()
	}
}
