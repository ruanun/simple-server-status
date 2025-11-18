package global

// 构建信息变量（由 -ldflags 在编译时注入）
var (
	BuiltAt   string
	GitCommit string
	Version   string = "dev"
	GoVersion string
)
