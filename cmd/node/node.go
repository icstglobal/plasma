package node

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/icstglobal/plasma/network"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NodeCmd represents the subcommand to start Plasma chain node
var NodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Start plasma node",
	Long:  `Start the Plasma child chain.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		n := new(network.Node)
		err := n.Start()
		if err != nil {
			log.WithError(err).Fatal("node failed to start")
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		waitForExit()
	},
}

func initConfig(cmd *cobra.Command) error {
	cfgFile := cmd.Flag("config").Value.String()
	// if confFile is not empty string, then it should has been loaded by the root command
	if cfgFile == "" {
		fmt.Println("config file not set, seaching for config files in current and $HOME dir")
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			// os.Exit(1)
			return err
		}

		// Search config in home directory with name ".plasma" (without extension).
		viper.AddConfigPath(".")
		viper.AddConfigPath(home)
		viper.SetConfigName(".plasma")

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		} else {
			fmt.Println("loading config failed:", err)
			return err
		}
	}

	// If a config file is found, read it in.
	slvl := viper.GetString("logger.level")
	if len(slvl) > 0 {
		if lvl, err := log.ParseLevel(slvl); err == nil {
			log.SetLevel(lvl)
			log.WithField("logger.level", slvl).Debug("change log level")
		} else {
			log.WithError(err).WithField("logger.level", slvl).Warn("unknown logger level")
		}
	}

	return nil
}

func waitForExit() {
	sch := make(chan os.Signal, 1)
	signal.Notify(sch, os.Interrupt)
	signal.Notify(sch, os.Kill)
	//wait
	<-sch

	log.Info("process being forced to quit")
	cleanup()

	//exit
	close(sch)
	log.Info("bye")
}

func cleanup() {
	//do anything before the process terminates
	log.Info("clean up...")
}
