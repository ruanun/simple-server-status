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
	ConfigFile = "sss-dashboard.yaml"
)

func InitConfig() {
	var config string

	flag.StringVar(&config, "c", "", "choose config file.")
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

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			//配置文件没有找到
			panic(fmt.Errorf("the config file does not exist: %s \n", err))
		} else {
			// 配置文件找到了,但是在这个过程有又出现别的什么error
			panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	}
	v.WatchConfig()

	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config file changed:", e.Name)
		if err := v.Unmarshal(&CONFIG); err != nil {
			fmt.Println(err)
		}
		ValidConfigAndConvert2Map()
	})
	if err := v.Unmarshal(&CONFIG); err != nil {
		fmt.Println(err)
	}
	ValidConfigAndConvert2Map()
}

func ValidConfigAndConvert2Map() {
	validate := validator.New()
	validErr := validate.Struct(CONFIG)
	if validErr != nil {
		LOG.Fatal(validErr)
	}
	if CONFIG.Port == 0 {
		CONFIG.Port = 8900
	}
	if CONFIG.Address == "" {
		CONFIG.Address = "0.0.0.0"
	}
	if CONFIG.WebSocketPath == "" {
		CONFIG.WebSocketPath = "ws-report"
	}
	//最小值5秒；
	if CONFIG.ReportTimeIntervalMax == 0 || CONFIG.ReportTimeIntervalMax < 5 {
		CONFIG.ReportTimeIntervalMax = 30
	}

	//模拟set数据类型
	temp := make(map[string]interface{})

	for _, v := range CONFIG.Servers {
		if _, ok := temp[v.Id]; ok {
			LOG.Fatal("配置文件中存在相同的服务器！", v.Id)
		}
		if v.Group == "" {
			v.Group = "DEFAULT"
		}

		temp[v.Id] = 1
		SERVERS.Set(v.Id, v)
	}
}
