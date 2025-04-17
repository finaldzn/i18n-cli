package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pandodao/i18n-cli/internal/config"
	"github.com/pandodao/i18n-cli/internal/gpt"
	"github.com/pandodao/i18n-cli/internal/scanner"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize translations across multiple files and languages",
	Long:  `Scan a directory structure for language files and synchronize translations from a source language to target languages.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get command flags
		rootDir, _ := cmd.Flags().GetString("root")
		sourceLang, _ := cmd.Flags().GetString("source")
		mode, _ := cmd.Flags().GetString("mode")
		batchSize, _ := cmd.Flags().GetInt("batch")
		configPath, _ := cmd.Flags().GetString("config")

		// Load configuration file if provided
		var cfg *config.Config
		var err error

		if configPath != "" {
			fmt.Printf("📝 Loading configuration from %s\n", configPath)
			cfg, err = config.LoadConfig(configPath)
			if err != nil {
				// If config file doesn't exist, create a default one
				if os.IsNotExist(err) {
					fmt.Printf("⚠️ Configuration file not found, creating default at %s\n", configPath)
					cfg = config.DefaultConfig()
					if err := config.SaveConfig(cfg, configPath); err != nil {
						fmt.Printf("❌ Error creating configuration file: %v\n", err)
						return
					}
				} else {
					fmt.Printf("❌ Error loading configuration: %v\n", err)
					return
				}
			}

			// Override with command line arguments if provided
			if cmd.Flags().Changed("source") {
				cfg.SourceLang = sourceLang
			} else {
				sourceLang = cfg.SourceLang
			}

			if cmd.Flags().Changed("mode") {
				cfg.Mode = mode
			} else {
				mode = cfg.Mode
			}

			if cmd.Flags().Changed("batch") {
				cfg.BatchSize = batchSize
			} else {
				batchSize = cfg.BatchSize
			}
		} else {
			// Use default config
			cfg = config.DefaultConfig()
			cfg.SourceLang = sourceLang
			cfg.Mode = mode
			cfg.BatchSize = batchSize
		}

		// Get API key from config or environment
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" && cfg.APIKey != "" {
			apiKey = cfg.APIKey
		}

		if apiKey == "" {
			fmt.Println("❌ No API key provided. Set OPENAI_API_KEY environment variable or specify in config file.")
			return
		}

		// Create GPT handler for translations
		gptHandler := gpt.New(gpt.Config{
			Keys:    []string{apiKey},
			Timeout: time.Duration(60) * time.Second,
		})

		// Create context
		ctx := context.Background()

		// Scan directory structure
		fmt.Printf("🔍 Scanning directory: %s (source: %s)\n", rootDir, sourceLang)
		ds, err := scanner.ScanDirectory(rootDir, sourceLang)
		if err != nil {
			fmt.Printf("❌ Error scanning directory: %v\n", err)
			return
		}

		fmt.Printf("✅ Found %d languages and %d file types\n", len(ds.Languages), len(ds.FileTypes))
		fmt.Printf("🌍 Languages: %v\n", ds.Languages)
		fmt.Printf("📄 File types: %v\n", ds.FileTypes)

		// Filter target languages if specified in config
		targetLanguages := []string{}
		if len(cfg.TargetLangs) > 0 {
			// Use only languages specified in config
			for _, lang := range ds.Languages {
				for _, targetLang := range cfg.TargetLangs {
					if lang == targetLang {
						targetLanguages = append(targetLanguages, lang)
						break
					}
				}
			}
			fmt.Printf("🎯 Using target languages from config: %v\n", targetLanguages)
		} else {
			// Use all languages except source
			for _, lang := range ds.Languages {
				if lang != sourceLang {
					targetLanguages = append(targetLanguages, lang)
				}
			}
		}

		// Check for missing files (files that exist in source but not in target)
		missingPairs := ds.FindMissingPairs()
		if len(missingPairs) > 0 {
			fmt.Printf("⚠️ Found %d missing files\n", len(missingPairs))
			for _, pair := range missingPairs {
				// Create target directory if it doesn't exist
				targetDir := filepath.Dir(pair.TargetFile)
				if _, err := os.Stat(targetDir); os.IsNotExist(err) {
					fmt.Printf("📁 Creating directory: %s\n", targetDir)
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						fmt.Printf("❌ Error creating directory: %v\n", err)
						continue
					}
				}
			}
		}

		// Get all file pairs
		pairs, err := ds.GetPairs()
		if err != nil {
			fmt.Printf("❌ Error getting file pairs: %v\n", err)
			return
		}

		// Filter pairs based on target languages
		filteredPairs := []scanner.FilePair{}
		for _, pair := range pairs {
			for _, lang := range targetLanguages {
				if pair.TargetLang == lang {
					filteredPairs = append(filteredPairs, pair)
					break
				}
			}
		}

		fmt.Printf("🔄 Processing %d file pairs\n", len(filteredPairs))

		// Statistics
		totalFiles := len(filteredPairs)
		completedFiles := 0
		totalKeys := 0
		translatedKeys := 0
		failedKeys := 0

		// Process each pair
		for _, pair := range filteredPairs {
			fmt.Printf("\n🔄 Processing: %s -> %s\n", pair.SourceFile, pair.TargetFile)

			// Load source and target files
			source, target, err := pair.LoadPair()
			if err != nil {
				fmt.Printf("❌ Error loading pair: %v\n", err)
				continue
			}

			// Create target directory if needed
			targetDir := filepath.Dir(pair.TargetFile)
			if _, err := os.Stat(targetDir); os.IsNotExist(err) {
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					fmt.Printf("❌ Error creating directory: %v\n", err)
					continue
				}
			}

			// Process the files
			var processErr error
			if batchSize > 0 {
				processErr = batch_process(ctx, gptHandler, source, target, nil, batchSize, mode)
			} else {
				processErr = single_process(ctx, gptHandler, source, target, nil, mode)
			}

			if processErr != nil {
				fmt.Printf("❌ Error processing pair: %v\n", processErr)
			}

			completedFiles++

			// Update statistics
			totalKeys += len(source.LocaleItemsMap)
			translatedCount := countTranslatedKeys(source.LocaleItemsMap, target.LocaleItemsMap)
			translatedKeys += translatedCount
			failedKeys += len(source.LocaleItemsMap) - translatedCount
		}

		// Print summary
		fmt.Printf("\n📊 Summary:\n")
		fmt.Printf("- Files processed: %d/%d\n", completedFiles, totalFiles)
		fmt.Printf("- Total keys: %d\n", totalKeys)
		fmt.Printf("- Translated keys: %d (%.1f%%)\n", translatedKeys, float64(translatedKeys)/float64(totalKeys)*100)
		fmt.Printf("- Failed keys: %d (%.1f%%)\n", failedKeys, float64(failedKeys)/float64(totalKeys)*100)

		fmt.Println("\n✅ Sync completed")
	},
}

// countTranslatedKeys counts how many keys in source have translations in target
func countTranslatedKeys(source, target map[string]string) int {
	count := 0
	for k := range source {
		if v, ok := target[k]; ok && v != "" {
			count++
		}
	}
	return count
}

func init() {
	syncCmd.Flags().String("root", "", "Root directory containing language subdirectories")
	syncCmd.Flags().String("source", "en", "Source language code (default: en)")
	syncCmd.Flags().StringVar(&translationMode, "mode", "missing", "Translation mode: 'full' (translate all) or 'missing' (only translate missing keys)")
	syncCmd.Flags().IntVar(&batchSize, "batch", 0, "Size of the batch for translations. If 0 or not provided, translates one at a time.")
	syncCmd.Flags().String("config", "", "Path to configuration file")

	syncCmd.MarkFlagRequired("root")

	rootCmd.AddCommand(syncCmd)
}
