package global

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"simple-server-status/agent/config"
)

var (
	BuiltAt   string
	GitCommit string
	Version   string = "dev"
	GoVersion string
)

var RetryCountMax = 999

var (
	AgentConfig *config.AgentConfig
	VP          *viper.Viper
	LOG         *zap.SugaredLogger
)

var (
	HostLocation string
	HostIp       string
)
