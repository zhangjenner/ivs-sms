package utils

import (
	"path/filepath"

	"github.com/go-ini/ini"
)

//=============================================================================
var (
	Conf     *ini.File
	ConfFile string
	Debug    bool
)

//=============================================================================

// LoadConf - 获取配置参数
func LoadConf() *ini.File {
	if Conf != nil {
		return Conf
	}
	ConfFile = filepath.Join(PWD(), "Conf.ini")
	if _conf, err := ini.InsensitiveLoad(ConfFile); err != nil {
		_conf, _ = ini.LoadSources(ini.LoadOptions{Insensitive: true}, []byte(""))
		Conf = _conf
	} else {
		Conf = _conf
	}
	return Conf
}

// ReloadConf - 重载配置文件
func ReloadConf() *ini.File {
	Conf = nil
	return LoadConf()
}

// SaveToConf - 保存配置文件
func SaveToConf(section string, kvmap map[string]string) error {
	var _conf *ini.File
	var err error
	if _conf, err = ini.InsensitiveLoad(ConfFile); err != nil {
		if _conf, err = ini.LoadSources(ini.LoadOptions{Insensitive: true}, []byte("")); err != nil {
			return err
		}
	}
	sec := _conf.Section(section)
	for k, v := range kvmap {
		sec.Key(k).SetValue(v)
	}
	_conf.SaveTo(ConfFile)
	Conf = _conf
	return nil
}
