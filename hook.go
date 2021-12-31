package zaptelegram

import (
	"errors"
	"github.com/go-co-op/gocron"
	"go.uber.org/zap/zapcore"
	"time"
)

const (
	defaultLevel       = zapcore.WarnLevel
	defaultAsyncOpt    = true
	defaultQueueOpt    = false
	defaultIntervalOpt = 1
)

var AllLevels = [6]zapcore.Level{
	zapcore.DebugLevel,
	zapcore.InfoLevel,
	zapcore.WarnLevel,
	zapcore.ErrorLevel,
	zapcore.FatalLevel,
	zapcore.PanicLevel,
}

var (
	TokenError   = errors.New("token not defined")
	ChatIDsError = errors.New("chat ids not defined")
)

type TelegramHook struct {
	telegramClient *telegramClient
	levels         []zapcore.Level
	async          bool
	queue          bool
	interval       int
	messages       []zapcore.Entry
}

func NewTelegramHook(token string, chatIDs []int, opts ...Option) (*TelegramHook, error) {
	if token == "" {
		return &TelegramHook{}, TokenError
	} else if len(chatIDs) == 0 {
		return &TelegramHook{}, ChatIDsError
	}
	c := newTelegramClient(token, chatIDs)
	h := &TelegramHook{
		telegramClient: c,
		levels:         []zapcore.Level{defaultLevel},
		async:          defaultAsyncOpt,
		interval:       defaultIntervalOpt,
		queue:          defaultQueueOpt,
	}
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func (h *TelegramHook) GetHook() func(zapcore.Entry) error {
	return func(e zapcore.Entry) error {
		if !h.isActualLevel(e.Level) {
			return nil
		}
		if h.async {
			go func() {
				_ = h.telegramClient.sendMessage(e)
			}()
			return nil
		} else if h.queue {
			h.pushMessage(e)
			return nil

		}
		if err := h.telegramClient.sendMessage(e); err != nil {
			return err
		}
		return nil
	}
}

func (h *TelegramHook) InitQueue() error {
	s := gocron.NewScheduler(time.UTC)
	_, err := s.Every(h.interval).Minutes().Do(func() {
		h.consume()
	})
	if err != nil {
		return err
	}

	s.StartAsync()
	return nil
}

func (h *TelegramHook) isActualLevel(l zapcore.Level) bool {
	for _, level := range h.levels {
		if level == l {
			return true
		}
	}
	return false
}

// Collect the log to send all at one time
func (h *TelegramHook) pushMessage(e zapcore.Entry) {
	h.messages = append(h.messages, e)
}

// Format all the logs then send to telegram (async)
func (h *TelegramHook) consume() {
	if len(h.messages) > 0 {
		go func() {
			messages := ""
			for _, message := range h.messages {
				messages += h.telegramClient.formatMessage(message) + "\n\n"
			}
			_ = h.telegramClient.sendMessages(messages)
		}()
	}
}

func getLevelThreshold(l zapcore.Level) []zapcore.Level {
	for i := range AllLevels {
		if AllLevels[i] == l {
			return AllLevels[i:]
		}
	}
	return []zapcore.Level{}
}
