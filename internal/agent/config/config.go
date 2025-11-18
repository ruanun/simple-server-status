package config

type AgentConfig struct {
	//服务器地址
	ServerAddr string `yaml:"serverAddr" validate:"required"`
	//每台机子对应id；唯一；在服务端配置
	ServerId string `yaml:"serverId" validate:"required"`
	//对应服务器配置的；做授权
	AuthSecret string `yaml:"authSecret" validate:"required"`
	//上报间隔，单位秒；默认2秒，最小值2
	ReportTimeInterval int `yaml:"reportTimeInterval"`
	//禁用根据IP查询服务器区域信息，默认false
	DisableIP2Region bool `yaml:"disableIP2Region"`

	//日志配置,日志级别
	LogPath string `yaml:"logPath"`
	//日志级别 debug,info,warn 默认info
	LogLevel string `yaml:"logLevel"`
}

// Validate 实现 ConfigLoader 接口 - 验证配置
func (c *AgentConfig) Validate() error {
	// 基础验证会在配置加载时自动完成
	// 详细验证在 main 函数中通过 ValidateAndSetDefaults 完成
	return nil
}

// OnReload 实现 ConfigLoader 接口 - 配置重载时的回调
func (c *AgentConfig) OnReload() error {
	// 配置重载后的处理在 app.LoadConfig 的回调中完成
	// 这里留空以避免循环导入
	return nil
}
