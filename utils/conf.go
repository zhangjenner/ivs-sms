package utils

import (
	"path/filepath"

	"github.com/go-ini/ini"
)

//=============================================================================
var (
	Conf  *ini.File
	Debug bool
)

//=============================================================================
// 包初始化
func init() {
	LoadConf()
	Debug = Conf.Section("base").Key("debug").MustBool()
}

//=============================================================================

// LoadConf - 获取配置参数
func LoadConf() *ini.File {
	cfile := filepath.Join(PWD(), "Conf.ini")
	conf, err := ini.InsensitiveLoad(cfile)
	if err != nil {
		conf, _ = ini.LoadSources(ini.LoadOptions{Insensitive: true}, []byte(""))
	}
	Conf = conf
	return conf
}

// SaveToConf - 保存配置文件
func SaveToConf(section string, kvmap map[string]string) (*ini.File, error) {
	var conf *ini.File
	var err error
	cfile := filepath.Join(PWD(), "Conf.ini")
	if conf, err = ini.InsensitiveLoad(cfile); err != nil {
		if conf, err = ini.LoadSources(ini.LoadOptions{Insensitive: true}, []byte("")); err != nil {
			return conf, err
		}
	}
	sec := conf.Section(section)
	for k, v := range kvmap {
		sec.Key(k).SetValue(v)
	}
	conf.SaveTo(cfile)
	Conf = conf
	return conf, nil
}
