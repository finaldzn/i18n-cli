package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pandodao/i18n-cli/internal/config"
	"github.com/pandodao/i18n-cli/internal/scanner"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show translation status",
	Long:  `Display the status of translations for all languages and files.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get command flags
		rootDir, _ := cmd.Flags().GetString("root")
		sourceLang, _ := cmd.Flags().GetString("source")
		configPath, _ := cmd.Flags().GetString("config")
		outputPath, _ := cmd.Flags().GetString("output")

		// Load configuration file if provided
		var cfg *config.Config
		var err error

		if configPath != "" {
			fmt.Printf("📝 Loading configuration from %s\n", configPath)
			cfg, err = config.LoadConfig(configPath)
			if err != nil && !os.IsNotExist(err) {
				fmt.Printf("❌ Error loading configuration: %v\n", err)
				return
			} else if err == nil {
				// Override with command line arguments if provided
				if !cmd.Flags().Changed("source") {
					sourceLang = cfg.SourceLang
				}
			}
		}

		// Scan directory structure
		fmt.Printf("🔍 Scanning directory: %s (source: %s)\n", rootDir, sourceLang)
		ds, err := scanner.ScanDirectory(rootDir, sourceLang)
		if err != nil {
			fmt.Printf("❌ Error scanning directory: %v\n", err)
			return
		}

		fmt.Printf("✅ Found %d languages and %d file types\n", len(ds.Languages), len(ds.FileTypes))

		// Filter target languages if specified in config
		targetLanguages := []string{}
		if cfg != nil && len(cfg.TargetLangs) > 0 {
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

		// Sort languages for consistent output
		sort.Strings(targetLanguages)

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

		// Group pairs by language and file type
		langFileStats := make(map[string]map[string]*FileStats)
		var totalSourceKeys int

		// First pass: collect source file key counts
		sourceKeyCounts := make(map[string]int)
		for _, pair := range filteredPairs {
			if _, ok := sourceKeyCounts[pair.FileType]; !ok {
				// Load source file to get the total number of keys
				source, _, err := pair.LoadPair()
				if err != nil {
					fmt.Printf("❌ Error loading source file %s: %v\n", pair.SourceFile, err)
					continue
				}
				sourceKeyCounts[pair.FileType] = len(source.LocaleItemsMap)
				totalSourceKeys += len(source.LocaleItemsMap)
			}
		}

		// Second pass: collect stats for each language and file
		for _, pair := range filteredPairs {
			// Initialize language map if needed
			if _, ok := langFileStats[pair.TargetLang]; !ok {
				langFileStats[pair.TargetLang] = make(map[string]*FileStats)
			}

			// Load source and target files
			source, target, err := pair.LoadPair()
			if err != nil {
				fmt.Printf("❌ Error loading pair: %v\n", err)
				continue
			}

			// Get missing keys
			missingKeys := findMissingKeys(source.LocaleItemsMap, target.LocaleItemsMap)
			missingCount := len(missingKeys)

			// Get empty keys (keys that exist but have empty values)
			emptyCount := 0
			for k, v := range target.LocaleItemsMap {
				if _, ok := source.LocaleItemsMap[k]; ok && v == "" {
					emptyCount++
				}
			}

			// Calculate statistics
			translatedCount := len(source.LocaleItemsMap) - missingCount - emptyCount
			percentComplete := float64(translatedCount) / float64(len(source.LocaleItemsMap)) * 100

			// Store statistics
			langFileStats[pair.TargetLang][pair.FileType] = &FileStats{
				SourceCount:   len(source.LocaleItemsMap),
				MissingCount:  missingCount,
				EmptyCount:    emptyCount,
				Translated:    translatedCount,
				PercentDone:   percentComplete,
				TargetExists:  true,
				TargetTooMany: len(target.LocaleItemsMap) > len(source.LocaleItemsMap),
			}
		}

		// Print results
		var output strings.Builder

		output.WriteString(fmt.Sprintf("# Translation Status Report\n\n"))
		output.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
		output.WriteString(fmt.Sprintf("Source Language: %s\n", sourceLang))
		output.WriteString(fmt.Sprintf("Target Languages: %d\n", len(targetLanguages)))
		output.WriteString(fmt.Sprintf("Total Source Keys: %d\n\n", totalSourceKeys))

		// Summary table header
		output.WriteString("## Summary\n\n")
		output.WriteString("| Language | Total Keys | Translated | Missing | Empty | Percent Complete |\n")
		output.WriteString("|----------|------------|------------|---------|-------|------------------|\n")

		// Overall stats by language
		for _, lang := range targetLanguages {
			if fileStats, ok := langFileStats[lang]; ok {
				totalKeys := 0
				totalTranslated := 0
				totalMissing := 0
				totalEmpty := 0

				for _, stats := range fileStats {
					totalKeys += stats.SourceCount
					totalTranslated += stats.Translated
					totalMissing += stats.MissingCount
					totalEmpty += stats.EmptyCount
				}

				percentComplete := float64(totalTranslated) / float64(totalKeys) * 100

				output.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %.1f%% |\n",
					lang, totalKeys, totalTranslated, totalMissing, totalEmpty, percentComplete))
			}
		}

		output.WriteString("\n## Details\n\n")

		// Detailed stats
		for _, lang := range targetLanguages {
			output.WriteString(fmt.Sprintf("### %s\n\n", lang))
			output.WriteString("| File | Total Keys | Translated | Missing | Empty | Percent Complete |\n")
			output.WriteString("|------|------------|------------|---------|-------|------------------|\n")

			if fileStats, ok := langFileStats[lang]; ok {
				// Get sorted file types
				fileTypes := make([]string, 0, len(fileStats))
				for fileType := range fileStats {
					fileTypes = append(fileTypes, fileType)
				}
				sort.Strings(fileTypes)

				for _, fileType := range fileTypes {
					stats := fileStats[fileType]
					output.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %.1f%% |\n",
						fileType, stats.SourceCount, stats.Translated, stats.MissingCount, stats.EmptyCount, stats.PercentDone))
				}
			}

			output.WriteString("\n")
		}

		// Print to console
		fmt.Println("\n" + output.String())

		// Save to file if requested
		if outputPath != "" {
			if err := os.WriteFile(outputPath, []byte(output.String()), 0644); err != nil {
				fmt.Printf("❌ Error writing output to file: %v\n", err)
			} else {
				fmt.Printf("✅ Report saved to %s\n", outputPath)
			}
		}
	},
}

// FileStats represents statistics for a file
type FileStats struct {
	SourceCount   int
	MissingCount  int
	EmptyCount    int
	Translated    int
	PercentDone   float64
	TargetExists  bool
	TargetTooMany bool
}

func init() {
	statusCmd.Flags().String("root", "", "Root directory containing language subdirectories")
	statusCmd.Flags().String("source", "en", "Source language code (default: en)")
	statusCmd.Flags().String("config", "", "Path to configuration file")
	statusCmd.Flags().String("output", "", "Save report to file (markdown format)")

	statusCmd.MarkFlagRequired("root")

	rootCmd.AddCommand(statusCmd)
}
