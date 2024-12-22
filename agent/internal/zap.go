package internal

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"simple-server-status/agent/global"
	"time"
)

var LevelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

var Logger *zap.Logger
var SugaredLogger *zap.SugaredLogger
var AtomicLevel zap.AtomicLevel

func InitLog() *zap.SugaredLogger {
	AtomicLevel = zap.NewAtomicLevelAt(LevelMap[global.AgentConfig.LogLevel])
	core := zapcore.NewCore(getEncoder(), getLogWriter(), AtomicLevel)

	Logger = zap.New(core, zap.AddCaller())
	SugaredLogger = Logger.Sugar()
	zap.ReplaceGlobals(Logger)

	SugaredLogger.Info("初始化日志模块成功")
	return SugaredLogger
}

func getLogWriter() zapcore.WriteSyncer {
	//这里我们使用zapcore.NewMultiWriteSyncer()实现同时输出到多个对象中
	writerSyncer := zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&lumberjack.Logger{
		Filename:  global.AgentConfig.LogPath, // ⽇志⽂件路径
		MaxSize:   64,                         // 单位为MB,默认为512MB
		MaxAge:    5,                          // 文件最多保存多少天
		LocalTime: true,                       // 采用本地时间
		Compress:  true,                       // 是否压缩日志
	}))
	return writerSyncer
}

func getEncoder() zapcore.Encoder {
	//自定义时间格式
	customTimeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	//自定义代码路径、行号输出
	customCallerEncoder := func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + caller.TrimmedPath() + "]")
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = customTimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeCaller = customCallerEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}
