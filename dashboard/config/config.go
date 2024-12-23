package config

type DashboardConfig struct {
	Address               string          `yaml:"address" json:"address"` //监听的地址；默认0.0.0.0
	Debug                 bool            `yaml:"debug" json:"debug"`
	Port                  int             `yaml:"port" json:"port"`                                    //监听的端口; 默认8900
	WebSocketPath         string          `yaml:"webSocketPath" json:"webSocketPath"`                  //agent WebSocket路径 默认ws-report
	ReportTimeIntervalMax int             `yaml:"reportTimeIntervalMax" json:"reportTimeIntervalMax "` //上报最大间隔；单位：秒 最小值5 默认值：30；离线判定，超过这个值既视为离线
	Servers               []*ServerConfig `yaml:"servers" validate:"required,dive,required" json:"servers"`

	//日志配置,日志级别
	LogPath  string `yaml:"logPath"`
	LogLevel string `yaml:"logLevel"`
}
