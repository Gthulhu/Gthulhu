package cmd

import (
	"context"
	"os"

	dmapp "github.com/Gthulhu/api/decisionmaker/app"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/spf13/cobra"
)

func init() {
	DMCmd.Flags().StringP("config-name", "c", "", "Configuration file name without extension")
	DMCmd.Flags().StringP("config-dir", "d", "", "Configuration file directory path")
}

// @title           manager service
// @version         1.0
// @description     manager service API documentation
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support

// @host      localhost:8081
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
	app, err := dmapp.NewRestApp(configName, configDirPath)
	if err != nil {
		logger.Logger(context.Background()).Fatal().Err(err).Msg("failed to create rest app")
	}
	app.Run()
}

func getConfigInfo(cmd *cobra.Command) (string, string) {
	configName := "dm_config"
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
	if envConfigName := os.Getenv("DM_CONFIG_NAME"); envConfigName != "" {
		configName = envConfigName
	}
	if envConfigPath := os.Getenv("DM_CONFIG_DIR_PATH"); envConfigPath != "" {
		configDirPath = envConfigPath
	}
	return configName, configDirPath
}

var DMCmd = &cobra.Command{
	Run: RunManagerApp,
	Use: "decisionmaker",
}
