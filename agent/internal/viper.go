package internal

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"simple-server-status/agent/global"
)

const (
	ConfigEnv       = "CONFIG"
	ConfigFile      = "sss-agent.yaml"
	DefaultLogLevel = "info"
	DefaultLogPath  = "./.logs/sss-agent.log"
)

func InitConfig() *viper.Viper {
	var config string

	flag.StringVarP(&config, "config", "c", "", "choose config file.")
	flag.StringP("serverAddr", "s", "", "server addr")
	flag.StringP("serverId", "i", "", "server id")
	flag.StringP("authSecret", "a", "", "auth Secret")
	flag.Int64P("reportTimeInterval", "t", 2, "report Time Interval")
	flag.BoolP("disableIP2Region", "r", false, "disable IP2Region")
	flag.StringP("logPath", "l", DefaultLogPath, "log path")
	flag.StringP("logLevel", "d", DefaultLogLevel, "log level debug|info|warn|error, default info")

	flag.Parse()
	if config == "" {
		// 优先级: 命令行 > 环境变量 > 默认值
		if configEnv := os.Getenv(ConfigEnv); configEnv == "" {
			config = ConfigFile
			fmt.Printf("您正在使用config的默认值,config的路径为%v\n", ConfigFile)
		} else {
			config = configEnv
			fmt.Printf("您正在使用CONFIG环境变量,config的路径为%v\n", config)
		}
	} else {
		fmt.Printf("您正在使用命令行的-c参数传递的值,config的路径为%v\n", config)
	}

	v := viper.New()
	v.SetConfigFile(config)
	v.SetConfigType("yaml")
	err := v.BindPFlags(flag.CommandLine)
	if err != nil {
		panic(err)
	}

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			//配置文件没有找到
			fmt.Printf("the config file does not exist: %s \n", err)
		} else {
			// 配置文件找到了,但是在这个过程有又出现别的什么error
			panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	}
	v.WatchConfig()

	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config file changed:", e.Name)
		if err := v.Unmarshal(&global.AgentConfig); err != nil {
			fmt.Println(err)
		}
		validConfig()
	})
	if err := v.Unmarshal(&global.AgentConfig); err != nil {
		fmt.Println(err)
	}

	validConfig()

	fmt.Println("初始化配置成功")
	return v
}

func validConfig() {
	validate := validator.New()
	validErr := validate.Struct(global.AgentConfig)
	if validErr != nil {
		panic(validErr)
	}

	if global.AgentConfig.ReportTimeInterval < 2 {
		global.AgentConfig.ReportTimeInterval = 2
	}
	if global.AgentConfig.LogPath == "" {
		global.AgentConfig.LogPath = DefaultLogPath
	}
	if global.AgentConfig.LogLevel == "" {
		global.AgentConfig.LogLevel = DefaultLogLevel
	}
}
