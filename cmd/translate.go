package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pandodao/i18n-cli/cmd/parser"
	"github.com/pandodao/i18n-cli/internal/gpt"

	"github.com/spf13/cobra"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var translateCmd = &cobra.Command{
	Use: "translate",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		apiKey := "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"
		if apiKey == "" {
			fmt.Println("environment variable OPENAI_API_KEY is empty")
			return
		}

		gptHandler := gpt.New(gpt.Config{
			Keys:    []string{apiKey},
			Timeout: time.Duration(60) * time.Second,
		})

		source, others, indep, err := provideFiles(cmd)
		if err != nil {
			cmd.PrintErrln("read files failed")
			return
		}

		cmd.Printf("📝 source: %d records\n", len(source.LocaleItemsMap))
		cmd.Println("🌐 Generating locale files:")

		if batchSize == 0 {
			for _, item := range others {
				err = single_process(ctx, gptHandler, source, item, indep, translationMode)
				if err != nil {
					cmd.PrintErrln("process failed: ", err)
					return
				}
			}
		} else {
			for _, item := range others {
				err = batch_process(ctx, gptHandler, source, item, indep, batchSize, translationMode)
				if err != nil {
					cmd.PrintErrln("process failed: ", err)
					return
				}
			}
		}
	},
}

// logTranslationError logs translation errors to a file for later analysis
func logTranslationError(key, sourceText, targetLang string, err error) {
	// Create logs directory if it doesn't exist
	logsDir := "translation_logs"
	if _, statErr := os.Stat(logsDir); os.IsNotExist(statErr) {
		os.Mkdir(logsDir, 0755)
	}

	// Create or open log file
	logFile := filepath.Join(logsDir, fmt.Sprintf("translation_errors_%s.log", time.Now().Format("2006-01-02")))
	f, fileErr := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		fmt.Printf("Error opening log file: %v\n", fileErr)
		return
	}
	defer f.Close()

	// Create a logger
	logger := log.New(f, "", log.LstdFlags)

	// Log the error with key, source text, target language, and error details
	errMsg := fmt.Sprintf("Key: %s\nSource: %s\nTarget Language: %s\nError: %v\n---\n",
		key, sourceText, targetLang, err)
	logger.Println(errMsg)
}

// logEmptyTranslation logs when we receive empty translations
func logEmptyTranslation(key, sourceText, targetLang string) {
	// Log as an error but with specific error type
	logTranslationError(key, sourceText, targetLang, fmt.Errorf("Empty translation received"))
}

func single_process(ctx context.Context, gptHandler *gpt.Handler, source *parser.LocaleFileContent, target *parser.LocaleFileContent, indep *parser.LocaleFileContent, mode string) error {
	count := 1
	failedKeys := []string{}

	// Find missing keys
	missingKeys := findMissingKeys(source.LocaleItemsMap, target.LocaleItemsMap)
	if len(missingKeys) > 0 {
		fmt.Printf("Found %d missing keys for %s\n", len(missingKeys), target.Path)
		for k := range missingKeys {
			if _, ok := source.LocaleItemsMap[k]; ok {
				target.LocaleItemsMap[k] = "" // Initialize with empty string to trigger translation
			}
		}
	}

	totalKeys := len(source.LocaleItemsMap)
	translatedCount := 0

	for k, v := range source.LocaleItemsMap {
		needToTranslate := false
		if len(v) != 0 {
			if _, ok := target.LocaleItemsMap[k]; !ok {
				// key does not exist, translate it
				needToTranslate = true
			} else {
				// key exists
				if indep != nil {
					if v, found := indep.LocaleItemsMap[k]; found {
						// key is in independent map, use the value in independent map
						target.LocaleItemsMap[k] = v
					}
				} else if mode == "full" {
					// In full mode, also translate empty strings and strings starting with "!"
					if len(target.LocaleItemsMap[k]) == 0 {
						// empty string, translate it
						needToTranslate = true
					} else if target.LocaleItemsMap[k][0] == '!' {
						// value starts with "!", translate it
						needToTranslate = true
					}
				} else if mode == "missing" {
					// In missing mode, only translate if the key is in the missing keys map
					_, isMissing := missingKeys[k]
					needToTranslate = isMissing
				}
			}

			if needToTranslate {
				var translationSuccess bool = true

				// Check if the value is a JSON array
				isValidJSONArray := false
				if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
					var stringArray []string
					if err := json.Unmarshal([]byte(v), &stringArray); err == nil {
						isValidJSONArray = true

						// This is actually a JSON array
						translatedArray := make([]string, len(stringArray))
						arrayTranslationFailed := false
						for i, str := range stringArray {
							translated, err := gptHandler.Translate(ctx, str, target.Lang)
							if err != nil {
								fmt.Printf("\n⚠️ Error translating array item in key %s: %v\n", k, err)
								logTranslationError(k, str, target.Lang, err)
								arrayTranslationFailed = true
								break
							}
							// Check for empty translations
							if translated == "" || translated == " " {
								fmt.Printf("\n⚠️ Empty translation for array item in key %s\n", k)
								logEmptyTranslation(k, str, target.Lang)
								arrayTranslationFailed = true
								break
							}
							translatedArray[i] = translated
						}

						if !arrayTranslationFailed {
							// Convert back to JSON string
							resultBytes, err := json.Marshal(translatedArray)
							if err != nil {
								fmt.Printf("\n⚠️ Error marshalling array for key %s: %v\n", k, err)
								logTranslationError(k, v, target.Lang, err)
								translationSuccess = false
							} else {
								target.LocaleItemsMap[k] = string(resultBytes)
							}
						} else {
							translationSuccess = false
						}
					}
				}

				// If not a valid JSON array, translate as a regular string
				if !isValidJSONArray {
					result, err := gptHandler.Translate(ctx, v, target.Lang)
					if err != nil {
						fmt.Printf("\n⚠️ Error translating key %s: %v\n", k, err)
						logTranslationError(k, v, target.Lang, err)
						translationSuccess = false
					} else if result == "" || result == " " {
						fmt.Printf("\n⚠️ Empty translation for key %s\n", k)
						logEmptyTranslation(k, v, target.Lang)
						translationSuccess = false
					} else {
						target.LocaleItemsMap[k] = result
					}
				}

				if translationSuccess {
					translatedCount++
				} else {
					failedKeys = append(failedKeys, k)
				}
			}

			fmt.Printf("\r🔄 %s: %d/%d (Translated: %d)", target.Path, count, totalKeys, translatedCount)
			count += 1
		}
	}

	// Report on failed translations
	if len(failedKeys) > 0 {
		fmt.Printf("\n⚠️ Failed to translate %d keys. You may want to run the command again or translate these manually.\n", len(failedKeys))
		if len(failedKeys) <= 10 {
			fmt.Println("Failed keys:", failedKeys)
		}

		// Save failed keys to a file for easier reference
		failedKeysFile := "failed_keys_" + filepath.Base(target.Path) + ".txt"
		content := strings.Join(failedKeys, "\n")
		os.WriteFile(failedKeysFile, []byte(content), 0644)
		fmt.Printf("Full list of failed keys saved to %s\n", failedKeysFile)
	}

	buf, err := target.JSON()
	if err != nil {
		return err
	}

	err = os.WriteFile(target.Path, buf, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("\r✅ %s: %d/%d (Translated: %d, Failed: %d)\n", target.Path, totalKeys, totalKeys, translatedCount, len(failedKeys))

	return nil
}

func batch_process(ctx context.Context, gptHandler *gpt.Handler, source *parser.LocaleFileContent, target *parser.LocaleFileContent, indep *parser.LocaleFileContent, batchSize int, mode string) error {
	var batch []string
	var keys []string
	var failedKeys []string

	// Find missing keys
	missingKeys := findMissingKeys(source.LocaleItemsMap, target.LocaleItemsMap)
	if len(missingKeys) > 0 {
		fmt.Printf("Found %d missing keys for %s\n", len(missingKeys), target.Path)
		for k := range missingKeys {
			if _, ok := source.LocaleItemsMap[k]; ok {
				target.LocaleItemsMap[k] = "" // Initialize with empty string to trigger translation
			}
		}
	}

	sendBatch := func() error {
		if len(batch) == 0 {
			return nil
		}

		results, err := gptHandler.BatchTranslate(ctx, batch, target.Lang)
		if err != nil {
			// Don't fail immediately, record the error and continue
			fmt.Printf("\n⚠️ Error translating batch: %v\n", err)

			// Log the error for each key in the batch
			for i, src := range batch {
				logTranslationError(keys[i], src, target.Lang, err)
				failedKeys = append(failedKeys, keys[i])
			}

			return err
		}

		for i, result := range results {
			// Check if the result is just a space or empty string (indicating a failed translation)
			if result == " " || result == "" {
				fmt.Printf("\n⚠️ Failed to translate key: %s\n", keys[i])
				logEmptyTranslation(keys[i], batch[i], target.Lang)
				failedKeys = append(failedKeys, keys[i])
				// Don't update the target with an empty value
				continue
			}
			target.LocaleItemsMap[keys[i]] = result
		}

		batch = batch[:0] // Clear the batch
		keys = keys[:0]   // Clear the keys
		return nil
	}

	count := 1
	totalKeys := len(source.LocaleItemsMap)
	translatedCount := 0

	for k, v := range source.LocaleItemsMap {
		needToTranslate := false
		if len(v) != 0 {
			if _, ok := target.LocaleItemsMap[k]; !ok {
				needToTranslate = true
			} else {
				if indep != nil {
					if v, found := indep.LocaleItemsMap[k]; found {
						target.LocaleItemsMap[k] = v
					}
				} else if mode == "full" {
					// In full mode, also check for empty strings and strings equal to source
					if strings.EqualFold(target.LocaleItemsMap[k], v) || len(target.LocaleItemsMap[k]) == 0 {
						needToTranslate = true
					} else if target.LocaleItemsMap[k][0] == '!' {
						needToTranslate = true
					}
				} else if mode == "missing" {
					// In missing mode, only translate if the key is in the missing keys map
					_, isMissing := missingKeys[k]
					needToTranslate = isMissing
				}
			}

			if needToTranslate {
				batch = append(batch, v)
				keys = append(keys, k)
				translatedCount++

				if len(batch) >= batchSize {
					// Process this batch, but don't return on error
					_ = sendBatch()
				}
			}

			fmt.Printf("\r🔄 %s: %d/%d (Translated: %d)", target.Path, count, totalKeys, translatedCount)
			count += 1
		}
	}

	// Process any remaining items
	if len(batch) > 0 {
		_ = sendBatch()
	}

	// Report on failed translations
	if len(failedKeys) > 0 {
		fmt.Printf("\n⚠️ Failed to translate %d keys. You may want to run the command again or translate these manually.\n", len(failedKeys))
		if len(failedKeys) <= 10 {
			fmt.Println("Failed keys:", failedKeys)
		}

		// Save failed keys to a file for easier reference
		failedKeysFile := "failed_keys_" + filepath.Base(target.Path) + ".txt"
		content := strings.Join(failedKeys, "\n")
		os.WriteFile(failedKeysFile, []byte(content), 0644)
		fmt.Printf("Full list of failed keys saved to %s\n", failedKeysFile)
	}

	buf, err := target.JSON()
	if err != nil {
		return err
	}

	err = os.WriteFile(target.Path, buf, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("\r✅ %s: %d/%d (Translated: %d, Failed: %d)\n", target.Path, totalKeys, totalKeys, translatedCount-len(failedKeys), len(failedKeys))
	return nil
}

func provideFiles(cmd *cobra.Command) (source *parser.LocaleFileContent, others []*parser.LocaleFileContent, indep *parser.LocaleFileContent, err error) {

	indepFile, err := cmd.Flags().GetString("independent")
	if err != nil {
		return
	}
	if indepFile != "" {
		indep = &parser.LocaleFileContent{}
		if err = indep.ParseFromJSONFile(indepFile); err != nil {
			return
		}
	}

	sourceFile, err := cmd.Flags().GetString("source")
	if err != nil {
		return
	}
	if sourceFile != "" {
		source = &parser.LocaleFileContent{}
		if err = source.ParseFromJSONFile(sourceFile); err != nil {
			return
		}

		var lang string
		lang, err = langCodeToName("en-US")
		if err != nil {
			return
		}

		source.Code = "en-US"
		source.Lang = lang
	} else {
		err = fmt.Errorf("source file is required")
		return
	}

	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return
	}
	if dir != "" {
		others = make([]*parser.LocaleFileContent, 0)
		items, _ := os.ReadDir(dir)
		sourceBaseFile := filepath.Base(sourceFile)
		for _, item := range items {
			if !item.IsDir() {
				name := filepath.Base(item.Name())
				ext := filepath.Ext(name)
				if strings.EqualFold(item.Name(), sourceBaseFile) {
					continue
				}

				if strings.ToLower(ext) != ".json" {
					fmt.Printf("file %s is not a JSON file. skip this file.\n", name)
					continue
				}

				localeContent := &parser.LocaleFileContent{}
				if err = localeContent.ParseFromJSONFile(path.Join(dir, item.Name())); err != nil {
					fmt.Println("parse file failed: ", err, ". skip this file.")
					continue
				}

				others = append(others, localeContent)
			}
		}
	} else {
		err = fmt.Errorf("dir is required")
		return
	}

	return
}

func langCodeToName(code string) (string, error) {
	tag, err := language.Parse(code)
	if err != nil {
		return "", err
	}
	return display.Self.Name(tag), nil
}

var batchSize int          // Declare a variable to hold the batch size
var translationMode string // Declare a variable to hold the translation mode

func init() {
	translateCmd.Flags().String("dir", "", "the directory of language files")
	translateCmd.Flags().String("source", "", "the source language file")
	translateCmd.Flags().String("independent", "", "the independent language file")
	translateCmd.Flags().IntVar(&batchSize, "batch", 0, "Size of the batch for translations. If 0 or not provided, translates one at a time.")
	translateCmd.Flags().StringVar(&translationMode, "mode", "full", "Translation mode: 'full' (translate all) or 'missing' (only translate missing keys)")

	rootCmd.AddCommand(translateCmd)
}

// Helper function to find missing keys in target compared to source
func findMissingKeys(source, target map[string]string) map[string]struct{} {
	missing := make(map[string]struct{})
	for k := range source {
		if _, exists := target[k]; !exists {
			missing[k] = struct{}{}
		}
	}
	return missing
}
