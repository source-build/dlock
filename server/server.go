package main

import (
	"flag"
	"github.com/source-build/dlock"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func init() {
	flag.Int("server.port", 7668, "service port not null")
	flag.String("server.secretKey", "", "service secretKey not null")
}

func main() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return
	}
	viper.SetConfigFile("etc/config.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		return
	}
	Server()
}

func Server() {
	dlock.InitLogger()

	dlock.StartLockStore()

	dlock.Run()
}
