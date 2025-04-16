# i18n-cli

A command-line tool for managing internationalization (i18n) in your applications, using AI for translation.

## Features

- Translate JSON locale files to multiple languages
- Support for nested JSON structures
- Batch processing for efficient translations
- Intelligent detection of missing translations (missing mode)
- Option to retranslate all keys (full mode)
- **NEW:** Synchronize entire locale directories (`sync` command)
- **NEW:** Generate translation status reports (`status` command)
- **NEW:** Configuration file support (`init` command)

## Installation

```bash
go install github.com/pandodao/i18n-cli@latest
```

## Basic Usage (`translate` command)

This command translates a single source file into multiple target files located in a directory.

```bash
# Example: Translate en-US.json to other json files in ./locales
i18n-cli translate --source ./locales/en-US.json --dir ./locales
```

### Translation Modes

-   `--mode missing` (Default in `sync`): Only translates keys that are in the source file but missing in the target file.
-   `--mode full` (Default in `translate`): Translates all keys from the source, overwriting existing translations in the target.

```bash
# Use missing mode
i18n-cli translate --source ./locales/en-US.json --dir ./locales --mode missing
```

### Batch Processing

Translate multiple strings at once for potentially faster processing:

```bash
i18n-cli translate --source ./locales/en-US.json --dir ./locales --batch 10
```

## Advanced Usage (New Commands)

### Directory Synchronization (`sync` command)

Manage translations across a standard directory structure where each language has its own subdirectory.

**Expected Directory Structure:**

```
locales/        <-- Your root directory
  ├── en/       <-- Source language directory
  │   ├── common.json
  │   └── translation.json
  ├── fr/       <-- Target language directory
  │   ├── common.json
  │   └── translation.json
  └── ...
```

**Usage:**

```bash
# Sync all files in ./locales, using 'en' as source, in missing mode
i18n-cli sync --root ./locales --source en --mode missing
```

### Configuration File (`init` and `--config`)

Manage settings like source/target languages, API key, batch size, and file patterns using a configuration file.

1.  **Create a config file:**
    ```bash
    i18n-cli init --config i18n-config.json
    ```
2.  **Edit `i18n-config.json`** to your needs.
    ```json
    {
      "sourceLang": "en",
      "targetLangs": ["fr", "es", "it"],
      "includeFiles": ["common.json", "translation.json"],
      "excludeFiles": [],
      "batchSize": 10,
      "mode": "missing",
      "apiKey": "YOUR_OPENAI_API_KEY"
    }
    ```
3.  **Use the config with `sync` or `status`:**
    ```bash
    i18n-cli sync --root ./locales --config i18n-config.json
    ```

### Translation Status (`status` command)

Generate a report showing the completion status for each language and file.

```bash
# Show status for ./locales, using 'en' as source
i18n-cli status --root ./locales --source en

# Save report to a file
i18n-cli status --root ./locales --config i18n-config.json --output report.md
```

## Environment Variables

-   `OPENAI_API_KEY`: Your OpenAI API key (can also be specified in the config file).

## Commands Reference

*   `i18n-cli translate [flags]`: Translate a single source file to multiple targets.
    *   `--source string`: Path to the source language file.
    *   `--dir string`: Directory containing target language files.
    *   `--mode string`: Translation mode: 'full' or 'missing' (default "full").
    *   `--batch int`: Batch size for translations (0 for single processing).
    *   `--independent string`: Path to an independent file with manual translations.
*   `i18n-cli sync [flags]`: Synchronize translations across a directory structure.
    *   `--root string`: Root directory containing language subdirectories.
    *   `--source string`: Source language code (default "en").
    *   `--mode string`: Translation mode: 'full' or 'missing' (default "missing").
    *   `--batch int`: Batch size (default 0).
    *   `--config string`: Path to configuration file.
*   `i18n-cli status [flags]`: Show translation status.
    *   `--root string`: Root directory.
    *   `--source string`: Source language code (default "en").
    *   `--config string`: Path to configuration file.
    *   `--output string`: Save report to a markdown file.
*   `i18n-cli init [flags]`: Initialize a configuration file.
    *   `--config string`: Path for the config file (default "i18n-config.json").
    *   `--force`: Overwrite existing config file.
    *   `--source string`: Default source language.
    *   `--targets strings`: Default target languages (comma-separated).

## License

MIT
