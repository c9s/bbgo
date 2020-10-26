package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/config"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/slack/slacklog"

	// import built-in strategies
	_ "github.com/c9s/bbgo/pkg/strategy/buyandhold"
	_ "github.com/c9s/bbgo/pkg/strategy/xpuremaker"
)

var errSlackTokenUndefined = errors.New("slack token is not defined.")

func init() {
	RunCmd.Flags().Bool("no-compile", false, "do not compile wrapper binary")
	RunCmd.Flags().String("os", runtime.GOOS, "GOOS")
	RunCmd.Flags().String("arch", runtime.GOARCH, "GOARCH")

	RunCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	RunCmd.Flags().String("since", "", "pnl since time")
	RootCmd.AddCommand(RunCmd)
}

var runTemplate = template.Must(template.New("main").Parse(`package main
// DO NOT MODIFY THIS FILE. THIS FILE IS GENERATED FOR IMPORTING STRATEGIES
import (
	"github.com/c9s/bbgo/pkg/cmd"

{{- range .Imports }}
	_ "{{ . }}"
{{- end }}
)

func main() {
	cmd.Execute()
}

`))

func compileRunFile(filepath string, config *config.Config) error {
	var buf = bytes.NewBuffer(nil)
	if err := runTemplate.Execute(buf, config); err != nil {
		return err
	}

	return ioutil.WriteFile(filepath, buf.Bytes(), 0644)
}

func runConfig(ctx context.Context, config *config.Config) error {
	slackToken := viper.GetString("slack-token")
	if len(slackToken) == 0 {
		return errSlackTokenUndefined
	}

	log.AddHook(slacklog.NewLogHook(slackToken, viper.GetString("slack-error-channel")))

	var notifier = slacknotifier.New(slackToken, viper.GetString("slack-channel"))

	db, err := cmdutil.ConnectMySQL()
	if err != nil {
		return err
	}

	environ := bbgo.NewDefaultEnvironment(db)
	environ.ReportTrade(notifier)

	trader := bbgo.NewTrader(environ)

	for _, entry := range config.ExchangeStrategies {
		for _, mount := range entry.Mounts {
			log.Infof("attaching strategy %T on %s...", entry.Strategy, mount)
			trader.AttachStrategyOn(mount, entry.Strategy)
		}
	}

	for _, strategy := range config.CrossExchangeStrategies {
		log.Infof("attaching strategy %T", strategy)
		trader.AttachCrossExchangeStrategy(strategy)
	}

	for _, report := range config.PnLReporters {
		if len(report.AverageCostBySymbols) > 0 {
			trader.ReportPnL(notifier).
				AverageCostBySymbols(report.AverageCostBySymbols...).
				Of(report.Of...).
				When(report.When...)
		} else {
			return errors.Errorf("unsupported PnL reporter: %+v", report)
		}
	}

	return trader.Run(ctx)
}

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "run strategies from config file",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		noCompile, err := cmd.Flags().GetBool("no-compile")
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		userConfig, err := config.Load(configFile)
		if err != nil {
			return err
		}

		// if there is no custom imports, we don't have to compile
		if noCompile || len(userConfig.Imports) == 0 {
			if err := runConfig(ctx, userConfig); err != nil {
				return err
			}
			cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
			return nil
		}

		var flagsArgs = []string{"run", "--no-compile"}
		cmd.Flags().Visit(func(flag *flag.Flag) {
			flagsArgs = append(flagsArgs, flag.Name, flag.Value.String())
		})
		flagsArgs = append(flagsArgs, args...)

		goOS, err := cmd.Flags().GetString("os")
		if err != nil {
			return err
		}

		goArch, err := cmd.Flags().GetString("arch")
		if err != nil {
			return err
		}

		var buildEnvs = []string{"GOOS=" + goOS, "GOARCH=" + goArch}
		return compileAndRun(ctx, userConfig, goOS, goArch, buildEnvs, flagsArgs...)
	},
}

func compile(buildDir string, userConfig *config.Config) error {
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		if err := os.MkdirAll(buildDir, 0777); err != nil {
			return errors.Wrapf(err, "can not create build directory: %s", buildDir)
		}
	}

	mainFile := filepath.Join(buildDir, "main.go")
	if err := compileRunFile(mainFile, userConfig); err != nil {
		return errors.Wrap(err, "compile error")
	}

	return nil
}

func compileAndRun(ctx context.Context, userConfig *config.Config, goOS, goArch string, buildEnvs []string, args ...string) error {
	buildDir := filepath.Join("build", "bbgow")

	if err := compile(buildDir, userConfig); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	buildTarget := filepath.Join(cwd, buildDir)
	log.Infof("building binary from %s...", buildTarget)

	binary := fmt.Sprintf("bbgow-%s-%s", goOS, goArch)

	buildCmd := exec.CommandContext(ctx, "go", "build", "-tags", "wrapper", "-o", binary, buildTarget)
	buildCmd.Env = append(os.Environ(), buildEnvs...)

	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return err
	}

	executePath := filepath.Join(cwd, binary)

	log.Infof("running wrapper binary, args: %v", args)
	runCmd := exec.CommandContext(ctx, executePath, args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return err
	}
	return nil
}
