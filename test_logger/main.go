package main

import (
	cherryLogger "github.com/cherry-game/cherry/logger"
)

func main() {
	config := &cherryLogger.Config{
		LogLevel:        "debug",
		StackLevel:      "error",
		EnableWriteFile: false,
		EnableConsole:   true,
		MaxAge:          0,
		TimeFormat:      "2006-01-02 15:04:05.000",
		PrintCaller:     false,
	}

	logger := cherryLogger.NewConfigLogger(config)

	logger.Info("111111111111111111111111111111")
	logger.Debugf("aaaaaaaaaaaaaa %s", "aaaaa args.......")
	logger.Infow("failed to fetch URL.", "url", "http://example.com")
	logger.Infow("failed to fetch URL.",
		"url", "http://example.com",
		"name", "url name",
	)
	logger.Warnw("failed to fetch URL.",
		"url", "http://example.com",
		"name", "url name",
	)
	logger.Errorw("failed to fetch URL.",
		"url", "http://example.com",
		"name", "url name",
	)
	logger.Fatal("fatal fatal fatal fatal fatal")

}
