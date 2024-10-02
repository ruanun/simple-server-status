package config

type ServerConfig struct {
	Name   string `yaml:"name" json:"name" validate:"required"`     //服务名字；展示使用
	Id     string `yaml:"id" json:"id" validate:"required"`         //id唯一
	Group  string `yaml:"group" json:"group"`                       //组
	Secret string `yaml:"secret" json:"secret" validate:"required"` //授权
}

type DashboardConfig struct {
	Address               string          `yaml:"address" json:"address"` //监听的地址；默认0.0.0.0
	Debug                 bool            `yaml:"debug" json:"debug"`
	Port                  int             `yaml:"port" json:"port"`                                    //监听的端口; 默认8900
	WebSocketPath         string          `yaml:"webSocketPath" json:"webSocketPath"`                  //agent WebSocket路径 默认ws-report
	ReportTimeIntervalMax int             `yaml:"reportTimeIntervalMax" json:"reportTimeIntervalMax "` //上报最大间隔；单位：秒 最小值5 默认值：30；离线判定，超过这个值既视为离线
	Servers               []*ServerConfig `yaml:"servers" validate:"required,dive,required" json:"servers"`
}
