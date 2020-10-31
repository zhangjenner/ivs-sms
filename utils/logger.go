package utils

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

//=============================================================================
var rl *rotatelogs.RotateLogs

//=============================================================================

// GetLogger - 获取日志记录器
func GetLogger() io.Writer {
	if rl != nil {
		return rl
	}
	logPath := filepath.Join(PWD(), Conf.Section("base").Key("log_path").MustString("logs"))
	logFile := filepath.Join(logPath, strings.ToLower("%Y%m%d.log"))
	_rl, err := rotatelogs.New(logFile, rotatelogs.WithMaxAge(-1), rotatelogs.WithRotationCount(3))
	if err == nil {
		rl = _rl
		return rl
	}
	log.Println("failed to create rotatelogs", err)
	log.Println("use stdout")
	return os.Stdout
}

// CloseLogger - 关闭日志记录器
func CloseLogger() {
	log.SetOutput(os.Stdout)
	if rl != nil {
		rl.Close()
		rl = nil
	}
}
