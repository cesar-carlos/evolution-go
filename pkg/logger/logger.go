package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/gomessguii/logger"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LoggerManager struct {
	config  *config.Config
	loggers map[string]*Logger
	mu      sync.RWMutex
}

type Logger struct {
	config     *config.Config
	instanceId string
	mu         sync.Mutex
	writer     *lumberjack.Logger
}

type LogEntry struct {
	Timestamp  time.Time       `json:"timestamp"`
	Level      string          `json:"level"`
	InstanceId string          `json:"instance_id"`
	Message    string          `json:"message"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

func NewLoggerManager(config *config.Config) *LoggerManager {
	// Garante que o diretório base de logs existe
	if err := os.MkdirAll(config.LogDirectory, 0755); err != nil {
		logger.LogError("Falha ao criar diretório base de logs: %v", err)
	}

	return &LoggerManager{
		config:  config,
		loggers: make(map[string]*Logger),
	}
}

func (lm *LoggerManager) GetLogger(instanceId string) *Logger {
	lm.mu.RLock()
	logger, exists := lm.loggers[instanceId]
	lm.mu.RUnlock()

	if exists {
		return logger
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Verificar novamente após obter o lock de escrita
	if logger, exists = lm.loggers[instanceId]; exists {
		return logger
	}

	// Criar novo logger para a instância
	logger = newLogger(instanceId, lm.config)
	lm.loggers[instanceId] = logger
	return logger
}

func newLogger(instanceId string, config *config.Config) *Logger {
	// Garante que o diretório existe
	logPath := filepath.Join(config.LogDirectory, instanceId)
	if err := os.MkdirAll(logPath, 0755); err != nil {
		logger.LogError("Falha ao criar diretório de logs: %v", err)
	}

	logFile := filepath.Join(logPath, "instance.log")

	writer := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    config.LogMaxSize,
		MaxBackups: config.LogMaxBackups,
		MaxAge:     config.LogMaxAge,
		Compress:   config.LogCompress,
	}

	return &Logger{
		config:     config,
		instanceId: instanceId,
		writer:     writer,
	}
}

func (l *Logger) LogInfo(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

func (l *Logger) LogError(format string, args ...interface{}) {
	logger.LogError("%s", l.log("ERROR", format, args...))
}

func (l *Logger) LogWarn(format string, args ...interface{}) {
	logger.LogWarn("%s", l.log("WARN", format, args...))
}

func (l *Logger) LogDebug(format string, args ...interface{}) {
	l.log("DEBUG", format, args...)
}

func (l *Logger) log(level string, format string, args ...interface{}) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	message := MaskSensitive(fmt.Sprintf(format, args...))
	entry := LogEntry{
		Timestamp:  time.Now(),
		Level:      level,
		InstanceId: l.instanceId,
		Message:    message,
	}

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		logger.LogError("Failed to marshal log entry: %v", err)
		return message
	}

	if _, err := l.writer.Write(append(jsonEntry, '\n')); err != nil {
		logger.LogError("Failed to write log: %v", err)
	}
	return message
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writer.Close()
}

// GetLogs retorna os logs da instância com filtros opcionais
func (l *Logger) GetLogs(startDate, endDate time.Time, level string, limit int) ([]LogEntry, error) {
	// Implementação movida para o service
	return nil, fmt.Errorf("método movido para instance_service")
}
