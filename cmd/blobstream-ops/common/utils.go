package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/rs/zerolog"
	tmconfig "github.com/tendermint/tendermint/config"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

// GetLogger creates a new logger and returns
func GetLogger(level string, format string) (tmlog.Logger, error) {
	logLvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level (%s): %w", level, err)
	}
	var logWriter io.Writer
	if strings.ToLower(format) == tmconfig.LogFormatPlain {
		logWriter = zerolog.ConsoleWriter{Out: os.Stderr}
	} else {
		logWriter = os.Stderr
	}

	return server.ZeroLogWrapper{Logger: zerolog.New(logWriter).Level(logLvl).With().Timestamp().Logger()}, nil
}

// TrapSignal will listen for any OS signal and cancel the context to exit gracefully.
func TrapSignal(logger tmlog.Logger, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGTERM)
	signal.Notify(sigCh, syscall.SIGINT)

	sig := <-sigCh
	logger.Info("caught signal; shutting down...", "signal", sig.String())
	cancel()
}

// ToEnvVariableFormat takes a flag and returns its corresponding environment
// variable.
// Example: if the flag is: `--flag1.flag2-flag3`, then, the environment variable that is looked for
// is `FLAG1_FLAG2_FLAG3`.
func ToEnvVariableFormat(flag string) string {
	if strings.HasPrefix(flag, "--") {
		flag = strings.TrimPrefix(flag, "--")
	}
	return strings.ReplaceAll(
		strings.ToUpper(strings.ReplaceAll(flag, "-", "_")),
		".",
		"_",
	)
}

func BindFlagAndEnvVar(cmd *cobra.Command, flag string) {
	if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := viper.BindEnv(flag, ToEnvVariableFormat(flag)); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
