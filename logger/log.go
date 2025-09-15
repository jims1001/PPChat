package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func init() {
	encCfg := zapcore.EncoderConfig{
		TimeKey:      "ts",
		LevelKey:     "level",
		NameKey:      "logger",
		CallerKey:    "caller",
		MessageKey:   "msg",
		LineEnding:   zapcore.DefaultLineEnding,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeLevel:  zapcore.CapitalColorLevelEncoder, // 彩色等级
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	Log = zap.New(core, zap.AddCaller())
}

// 快捷方法
func Info(msg string, fields ...zap.Field) { Log.Info(msg, fields...) }
func Infof(format string, args ...interface{}) {
	Log.Info(fmt.Sprintf(format, args...))
}
func Warn(msg string, fields ...zap.Field)  { Log.Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { Log.Error(msg, fields...) }

func Errorf(format string, args ...interface{}) {
	Log.Error(fmt.Sprintf(format, args...))
}

func Debug(msg string, fields ...zap.Field) { Log.Debug(msg, fields...) }
