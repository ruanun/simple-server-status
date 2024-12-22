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

	//日志配置,日志级别
	LogPath  string `yaml:"logPath"`
	LogLevel string `yaml:"logLevel"`
}
