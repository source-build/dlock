package dlock

import (
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var logger *logrus.Logger

func InitLogger() {
	if logger != nil {
		return
	}

	l := logrus.New()
	l.SetOutput(&lumberjack.Logger{
		Filename:   viper.GetString("log.path") + "/" + viper.GetString("log.fileName") + ".log",
		MaxSize:    viper.GetInt("log.FileMaxSize"),
		MaxBackups: viper.GetInt("log.MaxBackups"),
		MaxAge:     viper.GetInt("log.MaxAge"),
		Compress:   viper.GetBool("log.Compress"),
	})
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)
	logger = l
}
