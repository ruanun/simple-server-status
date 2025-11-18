package config

type DashboardConfig struct {
	Address               string          `yaml:"address" json:"address"` //监听的地址；默认0.0.0.0
	Debug                 bool            `yaml:"debug" json:"debug"`
	Port                  int             `yaml:"port" json:"port"`                                   //监听的端口; 默认8900
	WebSocketPath         string          `yaml:"webSocketPath" json:"webSocketPath"`                 //agent WebSocket路径 默认ws-report
	ReportTimeIntervalMax int             `yaml:"reportTimeIntervalMax" json:"reportTimeIntervalMax"` //上报最大间隔；单位：秒 最小值5 默认值：30；离线判定，超过这个值既视为离线
	Servers               []*ServerConfig `yaml:"servers" validate:"required,dive,required" json:"servers"`

	//日志配置,日志级别
	LogPath  string `yaml:"logPath"`
	LogLevel string `yaml:"logLevel"`
}

// Validate 实现 ConfigLoader 接口 - 验证配置
func (c *DashboardConfig) Validate() error {
	// 基础验证会在配置加载时自动完成
	// 详细验证在 main 函数中通过 ValidateAndApplyDefaults 完成
	return nil
}

// OnReload 实现 ConfigLoader 接口 - 配置重载时的回调
func (c *DashboardConfig) OnReload() error {
	// 配置重载后的处理在 app.LoadConfig 的回调中完成
	// 这里留空以避免循环导入
	return nil
}
