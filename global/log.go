package global

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newColorZap() *zap.Logger {
	encCfg := zapcore.EncoderConfig{
		TimeKey:       "ts",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stack",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeLevel:   zapcore.CapitalColorLevelEncoder, // 关键：彩色级别
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)
	return zap.New(core, zap.AddCaller())
}
