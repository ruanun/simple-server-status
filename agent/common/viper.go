package common

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
)

const (
	ConfigEnv  = "CONFIG"
	ConfigFile = "sss-agent.yaml"
)

func InitConfig() {

	var config string

	flag.StringVarP(&config, "config", "c", "", "choose config file.")
	flag.StringP("serverAddr", "s", "", "server addr")
	flag.StringP("serverId", "i", "", "server id")
	flag.StringP("authSecret", "a", "", "auth Secret")
	flag.Int64P("reportTimeInterval", "t", 2, "report Time Interval")

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
		return
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			//配置文件没有找到
			panic(fmt.Errorf("the config file does not exist: %s \n", err))
		} else {
			// 配置文件找到了,但是在这个过程有又出现别的什么error
			//panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	}
	v.WatchConfig()

	v.OnConfigChange(func(e fsnotify.Event) {
		LOG.Info("config file changed:", e.Name)
		if err := v.Unmarshal(&AgentConfig); err != nil {
			fmt.Println(err)
		}

		validConfig()
		//配置变更后刷新
		InitHeader()
	})
	if err := v.Unmarshal(&AgentConfig); err != nil {
		fmt.Println(err)
	}

	validConfig()
}

func validConfig() {
	validate := validator.New()
	validErr := validate.Struct(AgentConfig)
	if validErr != nil {
		LOG.Fatal(validErr)
	}

	if AgentConfig.ReportTimeInterval < 2 {
		AgentConfig.ReportTimeInterval = 2
	}
}
