package izap

import (
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 适配器
const (
	AdapterConsole = "console"
	AdapterFile    = "file"
)

// Config 日志配置
type Config struct {
	Adapter string `yaml:"adapter" json:"adapter"` // 适配器,默认console
	Level   int    `yaml:"level" json:"level"`     // 日志等级,默认info(1)
	// see lumberjack.Logger
	FileName   string `yaml:"fileName" json:"fileName"`     // 文件名,空字符使用默认     默认<processname>-lumberjack.log
	MaxSize    int    `yaml:"maxSize" json:"maxSize"`       // 每个日志文件最大尺寸(MB)  默认100MB,
	MaxAge     int    `yaml:"maxAge" json:"maxAge"`         // 日志文件保存天数         默认0不删除
	MaxBackups int    `yaml:"maxBackups" json:"maxBackups"` // 日志文件保存备份数        默认0都保存
	LocalTime  bool   `yaml:"localTime" json:"localTime"`   // 是否格式化时间戳         默认UTC时间
	Compress   bool   `yaml:"compress" json:"compress"`     // 压缩文件,采用gzip      默认不压缩
	Stack      bool   `yaml:"stack" json:"stack"`           // 使能栈调试输出
}

// 日志等级
var level = zap.NewAtomicLevel()

// InitLogger 初始化日志
func InitLogger(cfg Config) error {
	var core zapcore.Core
	var options []zap.Option
	level.SetLevel(zapcore.Level(cfg.Level)) // 设置日志输出等级

	switch cfg.Adapter {
	case AdapterFile: // 文件
		encodeCfg := zap.NewProductionEncoderConfig()     // 基础配置
		encodeCfg.EncodeTime = zapcore.ISO8601TimeEncoder // 修改输出时间格式
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encodeCfg),
			zapcore.AddSync(&lumberjack.Logger{ // 文件切割
				Filename:  cfg.FileName,
				MaxSize:   cfg.MaxSize,
				LocalTime: cfg.LocalTime,
				Compress:  cfg.Compress}),
			level)
	default: // 控制台
		encodeCfg := zap.NewDevelopmentEncoderConfig()    // 基础配置
		encodeCfg.EncodeTime = zapcore.ISO8601TimeEncoder // 修改输出时间格式
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encodeCfg),
			os.Stdout,
			level)

		stackLevel := zap.NewAtomicLevel()
		stackLevel.SetLevel(zap.WarnLevel) // 只显示栈的错误等级
		// 添加显示文件名和行号,跳过封装调用层,栈调用,及使能等级
		if cfg.Stack {
			options = append(options,
				zap.AddCaller(),
				zap.AddCallerSkip(1),
				zap.AddStacktrace(stackLevel),
			)
		}
	}
	l := zap.New(core, options...)
	zap.ReplaceGlobals(l)
	Logger = logger{l}
	return nil
}

// SetLevel 设置zap默认目志等级,线程安全
func SetLevel(l zapcore.Level) {
	level.SetLevel(l)
}

// Level get logger level
func Level() zapcore.Level {
	return level.Level()
}
