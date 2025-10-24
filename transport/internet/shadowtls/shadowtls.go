package shadowtls

import "context"

//go:generate go run github.com/frogwall/v2ray-core/v5/common/errors/errorgen

const protocolName = "shadowtls"

// stLogger implements shadowtls logger.ContextLogger interface
type stLogger struct{}

func (l *stLogger) Trace(args ...any) {}
func (l *stLogger) Debug(args ...any) {
	newError(args...).AtDebug().WriteToLog()
}
func (l *stLogger) Info(args ...any) {
	newError(args...).AtInfo().WriteToLog()
}
func (l *stLogger) Warn(args ...any) {
	newError(args...).AtWarning().WriteToLog()
}
func (l *stLogger) Error(args ...any) {
	newError(args...).AtError().WriteToLog()
}
func (l *stLogger) Fatal(args ...any) {
	newError(args...).AtError().WriteToLog()
}
func (l *stLogger) Panic(args ...any) {
	newError(args...).AtError().WriteToLog()
}

func (l *stLogger) TraceContext(ctx context.Context, args ...any) {}
func (l *stLogger) DebugContext(ctx context.Context, args ...any) {
	newError(args...).AtDebug().WriteToLog()
}
func (l *stLogger) InfoContext(ctx context.Context, args ...any) {
	newError(args...).AtInfo().WriteToLog()
}
func (l *stLogger) WarnContext(ctx context.Context, args ...any) {
	newError(args...).AtWarning().WriteToLog()
}
func (l *stLogger) ErrorContext(ctx context.Context, args ...any) {
	newError(args...).AtError().WriteToLog()
}
func (l *stLogger) FatalContext(ctx context.Context, args ...any) {
	newError(args...).AtError().WriteToLog()
}
func (l *stLogger) PanicContext(ctx context.Context, args ...any) {
	newError(args...).AtError().WriteToLog()
}
