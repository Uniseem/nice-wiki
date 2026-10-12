package config

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"net"
	"os"
	"strconv"
)

type Bootstrap struct {
	Data     string `toml:"-"`
	Address  string `toml:"address"`
	Dev      bool   `toml:"dev"`
	LogLevel string `toml:"log_level"`
}
type Overrides struct {
	Data, Address, LogLevel *string
	Dev                     *bool
}

// Load 的优先级为默认值、配置文件、环境变量、显式命令行参数。
type DataResolver func(string) (string, string, error)

func Load(flags Overrides, resolve DataResolver) (Bootstrap, error) {
	data := os.Getenv("NICE_WIKI_DATA")
	if data == "" {
		data = "data"
	}
	if flags.Data != nil {
		data = *flags.Data
	}
	root, bootstrapPath, err := resolve(data)
	if err != nil {
		return Bootstrap{}, err
	}
	result := Bootstrap{Data: root, Address: ":8080", LogLevel: "info"}
	if _, err := os.Stat(bootstrapPath); err == nil {
		metadata, err := toml.DecodeFile(bootstrapPath, &result)
		if err != nil {
			return result, fmt.Errorf("读取启动配置: %w", err)
		}
		if len(metadata.Undecoded()) != 0 {
			return result, errors.New("启动配置包含未知字段")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	if v := os.Getenv("NICE_WIKI_ADDR"); v != "" {
		result.Address = v
	}
	if v := os.Getenv("NICE_WIKI_LOG_LEVEL"); v != "" {
		result.LogLevel = v
	}
	if v := os.Getenv("NICE_WIKI_DEV"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return result, errors.New("NICE_WIKI_DEV 无效")
		}
		result.Dev = parsed
	}
	if flags.Address != nil {
		result.Address = *flags.Address
	}
	if flags.LogLevel != nil {
		result.LogLevel = *flags.LogLevel
	}
	if flags.Dev != nil {
		result.Dev = *flags.Dev
	}
	if _, _, err := net.SplitHostPort(result.Address); err != nil {
		return result, errors.New("监听地址必须包含端口")
	}
	switch result.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return result, errors.New("日志级别无效")
	}
	return result, nil
}
