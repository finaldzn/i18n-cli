package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pandodao/i18n-cli/cmd/parser"
)

// DirectoryStructure represents the structure of a localization directory
type DirectoryStructure struct {
	RootDir       string
	SourceLang    string
	Languages     []string
	FileTypes     []string
	LanguageDirs  map[string]string   // Map of language code to directory
	FilesByType   map[string][]string // Map of file type to files
	LanguageFiles map[string][]string // Map of language code to files
}

// ScanDirectory scans a directory for language files
func ScanDirectory(rootDir string, sourceLang string) (*DirectoryStructure, error) {
	// Check if directory exists
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", rootDir)
	}

	ds := &DirectoryStructure{
		RootDir:       rootDir,
		SourceLang:    sourceLang,
		Languages:     []string{},
		FileTypes:     []string{},
		LanguageDirs:  make(map[string]string),
		FilesByType:   make(map[string][]string),
		LanguageFiles: make(map[string][]string),
	}

	// List all subdirectories (language directories)
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	// First, find all language directories
	for _, entry := range entries {
		if entry.IsDir() {
			langCode := entry.Name()
			langPath := filepath.Join(rootDir, langCode)
			ds.Languages = append(ds.Languages, langCode)
			ds.LanguageDirs[langCode] = langPath
			ds.LanguageFiles[langCode] = []string{}
		}
	}

	// Make sure source language exists
	if _, exists := ds.LanguageDirs[sourceLang]; !exists {
		return nil, fmt.Errorf("source language directory '%s' not found", sourceLang)
	}

	// Scan source language directory to identify file types
	sourceFiles, err := os.ReadDir(ds.LanguageDirs[sourceLang])
	if err != nil {
		return nil, err
	}

	// Identify all JSON files in source directory
	for _, file := range sourceFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			fileType := file.Name()
			ds.FileTypes = append(ds.FileTypes, fileType)
			ds.FilesByType[fileType] = []string{}
		}
	}

	// Now scan all language directories for matching file types
	for lang, langDir := range ds.LanguageDirs {
		files, err := os.ReadDir(langDir)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				filePath := filepath.Join(langDir, file.Name())
				// Add file to language files
				ds.LanguageFiles[lang] = append(ds.LanguageFiles[lang], filePath)
				// Add file to file types
				ds.FilesByType[file.Name()] = append(ds.FilesByType[file.Name()], filePath)
			}
		}
	}

	return ds, nil
}

// GetPairs returns pairs of source and target files that need to be processed
func (ds *DirectoryStructure) GetPairs() ([]FilePair, error) {
	pairs := []FilePair{}

	// For each language except the source
	for _, lang := range ds.Languages {
		if lang == ds.SourceLang {
			continue
		}

		// For each file type
		for _, fileType := range ds.FileTypes {
			// Get source file path
			sourcePath := filepath.Join(ds.LanguageDirs[ds.SourceLang], fileType)
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				// Source file doesn't exist, skip
				continue
			}

			// Get or create target file path
			targetPath := filepath.Join(ds.LanguageDirs[lang], fileType)

			// Create the pair
			pair := FilePair{
				SourceFile: sourcePath,
				TargetFile: targetPath,
				SourceLang: ds.SourceLang,
				TargetLang: lang,
				FileType:   fileType,
			}

			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}

// FilePair represents a pair of source and target files
type FilePair struct {
	SourceFile string
	TargetFile string
	SourceLang string
	TargetLang string
	FileType   string
}

// LoadPair loads a pair of source and target files
func (fp *FilePair) LoadPair() (*parser.LocaleFileContent, *parser.LocaleFileContent, error) {
	source := &parser.LocaleFileContent{}

	// Skip language validation for directory-based paths
	source.Code = fp.SourceLang
	source.Lang = fp.SourceLang // Use code as language directly
	source.Path = fp.SourceFile

	// Read file content
	if err := source.ParseContent(); err != nil {
		return nil, nil, fmt.Errorf("error parsing source file %s: %w", fp.SourceFile, err)
	}

	target := &parser.LocaleFileContent{}

	if _, err := os.Stat(fp.TargetFile); os.IsNotExist(err) {
		// Target file doesn't exist, create empty map
		target.LocaleItemsMap = make(map[string]string)
		target.Path = fp.TargetFile
		target.Code = fp.TargetLang
		target.Lang = fp.TargetLang // Use code as language directly
	} else {
		// Target file exists
		target.Code = fp.TargetLang
		target.Lang = fp.TargetLang // Use code as language directly
		target.Path = fp.TargetFile

		// Read file content
		if err := target.ParseContent(); err != nil {
			return nil, nil, fmt.Errorf("error parsing target file %s: %w", fp.TargetFile, err)
		}
	}

	return source, target, nil
}

// FindMissingPairs finds file types that exist in source language but are missing in target languages
func (ds *DirectoryStructure) FindMissingPairs() []FilePair {
	missing := []FilePair{}

	for _, lang := range ds.Languages {
		if lang == ds.SourceLang {
			continue
		}

		for _, fileType := range ds.FileTypes {
			sourcePath := filepath.Join(ds.LanguageDirs[ds.SourceLang], fileType)
			targetPath := filepath.Join(ds.LanguageDirs[lang], fileType)

			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				// Source file doesn't exist, skip
				continue
			}

			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				// Target file doesn't exist, add to missing
				pair := FilePair{
					SourceFile: sourcePath,
					TargetFile: targetPath,
					SourceLang: ds.SourceLang,
					TargetLang: lang,
					FileType:   fileType,
				}
				missing = append(missing, pair)
			}
		}
	}

	return missing
}
