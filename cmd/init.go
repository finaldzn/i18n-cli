package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pandodao/i18n-cli/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long:  `Create a new configuration file for i18n-cli with default settings that you can customize.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if configPath == "" {
			configPath = "i18n-config.json"
		}

		// Check if file already exists
		if _, err := os.Stat(configPath); err == nil {
			override, _ := cmd.Flags().GetBool("force")
			if !override {
				fmt.Printf("⚠️ Configuration file %s already exists. Use --force to override.\n", configPath)
				return
			}
			fmt.Printf("⚠️ Overriding existing configuration file %s\n", configPath)
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(configPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("❌ Error creating directory: %v\n", err)
				return
			}
		}

		// Create default config
		cfg := config.DefaultConfig()

		// Set values from flags
		sourceLang, _ := cmd.Flags().GetString("source")
		if sourceLang != "" {
			cfg.SourceLang = sourceLang
		}

		targetLangs, _ := cmd.Flags().GetStringSlice("targets")
		if len(targetLangs) > 0 {
			cfg.TargetLangs = targetLangs
		}

		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey != "" {
			cfg.APIKey = apiKey
		}

		// Save config
		if err := config.SaveConfig(cfg, configPath); err != nil {
			fmt.Printf("❌ Error saving configuration: %v\n", err)
			return
		}

		fmt.Printf("✅ Configuration file created at %s\n", configPath)
		fmt.Println("📝 Edit this file to customize your translation settings")
		fmt.Println("💡 You can now use the sync command with --config flag:")
		fmt.Printf("   i18n-cli sync --root=./locales --config=%s\n", configPath)
	},
}

func init() {
	initCmd.Flags().String("config", "i18n-config.json", "Path to configuration file")
	initCmd.Flags().Bool("force", false, "Override existing configuration file")
	initCmd.Flags().String("source", "en", "Source language code")
	initCmd.Flags().StringSlice("targets", []string{}, "Target language codes (comma-separated)")

	rootCmd.AddCommand(initCmd)
}
