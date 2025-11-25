package main

import (
	"context"
	"log"
	"os"

	managerapp "github.com/Gthulhu/api/manager/app"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{Use: "manager or decisionmaker"}
)

func init() {
	ManagerCmd.Flags().StringP("config-name", "c", "", "Configuration file name without extension")
	ManagerCmd.Flags().StringP("config-dir", "d", "", "Configuration file directory path")
}

func main() {
	rootCmd.AddCommand(ManagerCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
		os.Exit(1)
	}
}

// GrpcServerCmd is the command to start the gRPC server
var ManagerCmd = &cobra.Command{
	Run: RunManagerApp,
	Use: "manager",
}

func RunManagerApp(cmd *cobra.Command, args []string) {
	configName, configDirPath := getConfigInfo(cmd)
	logger.InitLogger()
	app, err := managerapp.NewRestApp(configName, configDirPath)
	if err != nil {
		logger.Logger(context.Background()).Fatal().Err(err).Msg("failed to create rest app")
	}
	app.Run()
}

func getConfigInfo(cmd *cobra.Command) (string, string) {
	configName := "manager_config"
	configDirPath := ""
	if cmd != nil {
		configNameFlag, err := cmd.Flags().GetString("config-name")
		if err == nil && configNameFlag != "" {
			configName = configNameFlag
		}
		configPathFlag, err := cmd.Flags().GetString("config-dir")
		if err == nil && configPathFlag != "" {
			configDirPath = configPathFlag
		}
	}
	if envConfigName := os.Getenv("MANAGER_CONFIG_NAME"); envConfigName != "" {
		configName = envConfigName
	}
	if envConfigPath := os.Getenv("MANAGER_CONFIG_DIR_PATH"); envConfigPath != "" {
		configDirPath = envConfigPath
	}
	return configName, configDirPath
}
