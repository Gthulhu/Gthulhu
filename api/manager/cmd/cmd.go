package cmd

import (
	"context"
	"os"

	managerapp "github.com/Gthulhu/api/manager/app"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/spf13/cobra"
)

func init() {
	ManagerCmd.Flags().StringP("config-name", "c", "", "Configuration file name without extension")
	ManagerCmd.Flags().StringP("config-dir", "d", "", "Configuration file directory path")
}

// @title           manager service
// @version         1.0
// @description     manager service API documentation
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support

// @host      localhost:8080
// @BasePath  /

// @Accept json
// @Produce json

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @schemes http

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
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

var ManagerCmd = &cobra.Command{
	Run: RunManagerApp,
	Use: "manager",
}
