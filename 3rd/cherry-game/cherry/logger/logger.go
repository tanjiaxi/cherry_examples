package cherryLogger

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	ccontext "github.com/cherry-game/cherry/extend/context"
	cfacade "github.com/cherry-game/cherry/facade"
	"github.com/cherry-game/cherry/logger/rotatelogs"
	cprofile "github.com/cherry-game/cherry/profile"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	rw              sync.RWMutex             // mutex
	DefaultLogger   *CherryLogger            // 默认日志对象(控制台输出)
	loggers         map[string]*CherryLogger // 日志实例存储map(key:日志名称,value:日志实例)
	nodeID          string                   // current node id
	printLevel      zapcore.Level            // cherry log print level
	fileNameVarMap  = map[string]string{}    // 日志输出文件名自定义变量
	bufferedWriters []*zapcore.BufferedWriteSyncer
)

func init() {
	DefaultLogger = NewConfigLogger(defaultConsoleConfig(), zap.AddCallerSkip(1))
	loggers = make(map[string]*CherryLogger)
}

type CherryLogger struct {
	*zap.SugaredLogger
	*zap.Logger
	*Config
}

func (c *CherryLogger) Print(v ...interface{}) {
	c.SugaredLogger.Warn(v)
}

func SetNodeLogger(node cfacade.INode) {
	nodeID = node.NodeID()
	refLoggerName := node.Settings().Get("ref_logger").ToString()
	if refLoggerName == "" {
		DefaultLogger.Infof("RefLoggerName not found, used default console logger.")
		return
	}

	SetFileNameVar("nodeid", node.NodeID())     // %nodeid
	SetFileNameVar("nodetype", node.NodeType()) // %nodetype

	DefaultLogger = NewLogger(refLoggerName, zap.AddCallerSkip(1))
	printLevel = GetLevel(cprofile.PrintLevel())
}

func SetFileNameVar(key, value string) {
	fileNameVarMap[key] = value
}

func Flush() {
	// _ = DefaultLogger.SugaredLogger.Sync()
	// _ = DefaultLogger.Logger.Sync()
	// for _, logger := range loggers {
	// 	_ = logger.SugaredLogger.Sync()
	// 	_ = logger.Logger.Sync()
	// }
	for _, bw := range bufferedWriters {
		bw.Stop() // 阻塞直到所有缓冲写入完成
	}
}

func NewLogger(refLoggerName string, opts ...zap.Option) *CherryLogger {
	if refLoggerName == "" {
		return nil
	}

	defer rw.Unlock()
	rw.Lock()

	if logger, found := loggers[refLoggerName]; found {
		return logger
	}

	config, err := NewConfigWithName(refLoggerName)
	if err != nil {
		Panicf("New Config fail. err = %v", err)
	}

	logger := NewConfigLogger(config, opts...)
	loggers[refLoggerName] = logger

	return logger
}

func NewConfigLogger(config *Config, opts ...zap.Option) *CherryLogger {
	if config.EnableWriteFile {
		for key, value := range fileNameVarMap {
			config.FileLinkPath = strings.ReplaceAll(config.FileLinkPath, "%"+key, value)
			config.FilePathFormat = strings.ReplaceAll(config.FilePathFormat, "%"+key, value)
		}
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		CallerKey:      "caller",
		MessageKey:     "msg",
		NameKey:        "name",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// encoderConfig.EncodeLevel = func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
	// 	if nodeID != "" {
	// 		encoder.AppendString(fmt.Sprintf("%s  %-5s", nodeID, level.CapitalString()))
	// 	} else {
	// 		encoder.AppendString(level.CapitalString())
	// 	}
	// }
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	if config.PrintCaller {
		encoderConfig.EncodeTime = config.TimeEncoder()
		encoderConfig.EncodeName = zapcore.FullNameEncoder
		encoderConfig.FunctionKey = zapcore.OmitKey
		opts = append(opts, zap.AddCaller())
	}

	opts = append(opts, zap.AddStacktrace(GetLevel(config.StackLevel)))

	var writers []zapcore.WriteSyncer

	if config.EnableWriteFile {
		hook, err := rotatelogs.New(
			config.FilePathFormat, //filename+"_%Y%m%d%H%M.log",
			rotatelogs.WithLinkName(config.FileLinkPath),
			rotatelogs.WithMaxAge(time.Hour*24*time.Duration(config.MaxAge)),
			rotatelogs.WithRotationTime(time.Second*time.Duration(config.RotationTime)),
		)

		if err != nil {
			panic(err)
		}
		// 包一层异步
		bw := &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(hook),
			Size:          256 * 1024,      // 256KB 缓冲区
			FlushInterval: 5 * time.Second, // 每 5 秒刷一次
		}
		bufferedWriters = append(bufferedWriters, bw)
		writers = append(writers, bw)
		//同步
		// writers = append(writers, zapcore.AddSync(hook))
	}

	if config.EnableConsole {
		bw := &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(os.Stderr),
			Size:          256 * 1024,
			FlushInterval: 5 * time.Second,
		}
		bufferedWriters = append(bufferedWriters, bw)
		writers = append(writers, bw)
		//同步
		// writers = append(writers, zapcore.AddSync(os.Stderr))
	}

	if config.IncludeStdout {
		bw := &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(os.Stdout),
			Size:          256 * 1024,
			FlushInterval: 5 * time.Second,
		}
		bufferedWriters = append(bufferedWriters, bw)
		writers = append(writers, bw)
		//同步
		// writers = append(writers, zapcore.AddSync(os.Stdout))
	}

	if config.IncludeStderr {
		bw := &zapcore.BufferedWriteSyncer{
			WS:            zapcore.AddSync(os.Stderr),
			Size:          256 * 1024,
			FlushInterval: 5 * time.Second,
		}
		bufferedWriters = append(bufferedWriters, bw)
		writers = append(writers, bw)
		//同步
		// writers = append(writers, zapcore.AddSync(os.Stderr))
	}

	var encoder zapcore.Encoder
	if config.EnableConsole { // 或者是你自定义的配置项，如 config.JsonFormat
		// encoder = zapcore.NewConsoleEncoder(encoderConfig)
		encoder = zapcore.NewJSONEncoder(encoderConfig) // 全行转为标准 JSON 字段
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig) // 全行转为标准 JSON 字段
	}
	core := zapcore.NewCore(
		encoder,
		// zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(zapcore.NewMultiWriteSyncer(writers...)),
		zap.NewAtomicLevelAt(GetLevel(config.LogLevel)),
	)
	// 修改为：
	zapLogger := zap.New(core, opts...)

	// 如果当前节点 ID 不为空，让这个 Logger 实例初始就自带 node_id 字段
	if nodeID != "" {
		zapLogger = zapLogger.With(zap.String("node_id", nodeID))
	}
	cherryLogger := &CherryLogger{
		SugaredLogger: zapLogger.Sugar(), // 转为糖衣日志
		Logger:        zapLogger,         // 专门给高性能核心链路使用
		Config:        config,
	}

	return cherryLogger
}

func NewSugaredLogger(core zapcore.Core, opts ...zap.Option) *zap.SugaredLogger {
	zapLogger := zap.New(core, opts...)
	return zapLogger.Sugar()
}

func Enable(level zapcore.Level) bool {
	return DefaultLogger.Desugar().Core().Enabled(level)
}

func Debug(args ...interface{}) {
	DefaultLogger.SugaredLogger.Debug(args...)
}

func Info(args ...interface{}) {
	DefaultLogger.SugaredLogger.Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	DefaultLogger.SugaredLogger.Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	DefaultLogger.SugaredLogger.Error(args...)
}

// DPanic uses fmt.Sprint to construct and log a message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanic(args ...interface{}) {
	DefaultLogger.SugaredLogger.DPanic(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	DefaultLogger.SugaredLogger.Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	DefaultLogger.SugaredLogger.Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	DefaultLogger.Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	DefaultLogger.Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	DefaultLogger.Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	DefaultLogger.Errorf(template, args...)
}

// DPanicf uses fmt.Sprintf to log a templated message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...interface{}) {
	DefaultLogger.DPanicf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	DefaultLogger.Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	DefaultLogger.Fatalf(template, args...)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//	s.With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	DefaultLogger.Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.Errorw(msg, keysAndValues...)
}

// DPanicw logs a message with some additional context. In development, the
// logger then panics. (See DPanicLevel for details.) The variadic key-value
// pairs are treated as they are in With.
func DPanicw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.DPanicw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics. The
// variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	DefaultLogger.Fatalw(msg, keysAndValues...)
}

func PrintLevel(level zapcore.Level) bool {
	return level >= printLevel
}

// ---- 以下是新增的带 Context 打印函数 ----

func DebugContext(ctx context.Context, msg string, fields ...zap.Field) {
	// 1. 提取 ctx 中的强类型 fields
	if ctxFields := extractContextFields(ctx); len(ctxFields) > 0 {
		// 2. 将 ctx 中的字段和传入的字段合并
		DefaultLogger.Logger.With(ctxFields...).Debug(msg, fields...)
	} else {
		// 3. 无 ctx 字段，直接打印传入字段
		DefaultLogger.Logger.Debug(msg, fields...)
	}
}
func InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	if ctxFields := extractContextFields(ctx); len(ctxFields) > 0 {
		DefaultLogger.Logger.With(ctxFields...).Info(msg, fields...)
	} else {
		DefaultLogger.Logger.Info(msg, fields...)
	}
}

func WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	if ctxFields := extractContextFields(ctx); len(ctxFields) > 0 {
		DefaultLogger.Logger.With(ctxFields...).Warn(msg, fields...)
	} else {
		DefaultLogger.Logger.Warn(msg, fields...)
	}
}

func ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {
	if ctxFields := extractContextFields(ctx); len(ctxFields) > 0 {
		DefaultLogger.Logger.With(ctxFields...).Error(msg, fields...)
	} else {
		DefaultLogger.Logger.Error(msg, fields...)
	}
}

// ---- 以下是对应的 格式化(f) 版本 ---- ,性能没有logger好

// func DebugfContext(ctx context.Context, msg string, args ...interface{}) {
// 	if fields := extractContextFields(ctx); len(fields) > 0 {
// 		DefaultLogger.With(args...).Debugf(template, args...)
// 	} else {
// 		DefaultLogger.Debugf(template, args...)
// 	}
// }

// func InfofContext(ctx context.Context, template string, args ...interface{}) {
// 	if fields := extractContextFields(ctx); len(fields) > 0 {
// 		DefaultLogger.With(args...).Infof(template, args...)
// 	} else {
// 		DefaultLogger.Infof(template, args...)
// 	}
// }

// func WarnfContext(ctx context.Context, template string, args ...interface{}) {
// 	if fields := extractContextFields(ctx); len(fields) > 0 {
// 		DefaultLogger.With(fields...).Warnf(template, args...)
// 	} else {
// 		DefaultLogger.Warnf(template, args...)
// 	}
// }

// func ErrorfContext(ctx context.Context, template string, args ...interface{}) {
// 	if fields := extractContextFields(ctx); len(fields) > 0 {
// 		DefaultLogger.With(fields...).Errorf(template, args...)
// 	} else {
// 		DefaultLogger.Errorf(template, args...)
// 	}
// }

// // ---- 如果需要带 KV 的 w 版本 ----

// func InfowContext(ctx context.Context, msg string, keysAndValues ...interface{}) {
// 	if fields := extractContextFields(ctx); len(fields) > 0 {
// 		// 合并 context 字段和传入的临时字段
// 		fullFields := append(fields, keysAndValues...)
// 		DefaultLogger.Infow(msg, fullFields...)
// 	} else {
// 		DefaultLogger.Infow(msg, keysAndValues...)
// 	}
// }

// func ErrorwContext(ctx context.Context, msg string, keysAndValues ...interface{}) {
// 	if fields := extractContextFields(ctx); len(fields) > 0 {
// 		fullFields := append(fields, keysAndValues...)
// 		DefaultLogger.Errorw(msg, fullFields...)
// 	} else {
// 		DefaultLogger.Errorw(msg, keysAndValues...)
// 	}
// }

func GetLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.DebugLevel
	}
}

// 辅助函数：将 Context 中的公共字段提取并转换为 zap 能够识别的键值对形式
func extractContextFields(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}

	commonCtx := ccontext.FromContext(ctx) // 如果你在当前包实现了 FromContext

	if commonCtx == nil {
		return nil
	}

	fields := make([]zap.Field, 0)

	// 1. 提取公共字段（永远在第一层） Logger 需要fields
	if ctx != nil {
		if commonCtx := ccontext.FromContext(ctx); commonCtx != nil {
			fields = append(fields, zap.String("trace_id", commonCtx.TraceId))
			fields = append(fields, zap.String("user_id", commonCtx.UserId))
		}
	}
	return fields
	// DefaultLogger.With(keysAndValues...).Desugar().Debug(msg, fields...)
}
