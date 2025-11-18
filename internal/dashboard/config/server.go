package config

type ServerConfig struct {
	Name        string `yaml:"name" json:"name" validate:"required"`     //服务名字；展示使用
	Id          string `yaml:"id" json:"id" validate:"required"`         //id唯一
	Group       string `yaml:"group" json:"group"`                       //组
	Secret      string `yaml:"secret" json:"secret" validate:"required"` //授权
	CountryCode string `yaml:"countryCode" json:"countryCode"`           //国家代码 CN JP US SG
}
