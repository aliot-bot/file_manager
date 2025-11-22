package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port          int   `yaml:"port"`
	MaxUploadSize int64 `yaml:"max_upload_size"`
}

type StorageConfig struct {
	BasePath string `yaml:"base_path"`
}

type StaticConfig struct {
	Path         string `yaml:"path"`
	TemplateFile string `yaml:"template_file"`
}

type FileConfig struct {
	MaxNameLength       int         `yaml:"max_name_length"`
	DirPermissions      os.FileMode `yaml:"dir_permissions"`
	ForbiddenExtensions []string    `yaml:"forbidden_extensions"`
	ValidNameRegex      string      `yaml:"valid_name_regex"`
}

type RoutesConfig struct {
	Browse         string `yaml:"browse"`
	BrowseAlt      string `yaml:"browse_alt"`
	Upload         string `yaml:"upload"`
	CreateFolder   string `yaml:"create_folder"`
	Delete         string `yaml:"delete"`
	Rename         string `yaml:"rename"`
	Download       string `yaml:"download"`
	DownloadFolder string `yaml:"download_folder"`
}

type Messages struct {
	CannotListDirectory string `yaml:"cannot_list_directory"`
	TemplateError       string `yaml:"template_error"`
	RenderError         string `yaml:"render_error"`
	ForbiddenFile       string `yaml:"forbidden_file"`
	CannotServe         string `yaml:"cannot_serve"`
	CannotDelete        string `yaml:"cannot_delete"`
	InternalError       string `yaml:"internal_error"`
}

type Config struct {
	Server   ServerConfig  `yaml:"server"`
	Storage  StorageConfig `yaml:"storage"`
	Static   StaticConfig  `yaml:"static"`
	File     FileConfig    `yaml:"file"`
	Routes   RoutesConfig  `yaml:"routes"`
	Messages Messages      `yaml:"messages"`
}

func LoadConfig(filename string) *Config {
	cfg, err := LoadConfigWithError(filename)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	return cfg
}

func LoadConfigWithError(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", unmarshalErr)
	}

	// делаю абсолютные пути относительными для стабильности, независимо от рабочего каталога.
	paths := map[string]*string{
		"storage base path": &cfg.Storage.BasePath,
		"static path":       &cfg.Static.Path,
	}

	for name, path := range paths {
		absPath, absErr := filepath.Abs(*path)
		if absErr != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", name, absErr)
		}
		*path = absPath
	}

	// валидация конфига
	if validationErr := validateConfig(&cfg); validationErr != nil {
		return nil, validationErr
	}

	return &cfg, nil
}

type validationError struct {
	field string
	msg   string
}

func (e validationError) Error() string {
	return fmt.Sprintf("%s: %s", e.field, e.msg)
}

func validateConfig(cfg *Config) error {
	type validator func() error

	validators := []validator{
		func() error { return validateRequiredString("storage.base_path", cfg.Storage.BasePath) },
		func() error { return validateRequiredString("static.path", cfg.Static.Path) },
		func() error { return validateRequiredString("static.template_file", cfg.Static.TemplateFile) },
		func() error { return validateRequiredString("file.valid_name_regex", cfg.File.ValidNameRegex) },
		func() error { return validatePort(cfg.Server.Port) },
		func() error { return validatePositiveInt64("server.max_upload_size", cfg.Server.MaxUploadSize) },
		func() error { return validatePositiveInt("file.max_name_length", cfg.File.MaxNameLength) },
	}

	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}

	return nil
}

func validateRequiredString(field, value string) error {
	if value == "" {
		return validationError{field: field, msg: "is required"}
	}
	return nil
}

func validatePositiveInt(field string, value int) error {
	if value <= 0 {
		return validationError{field: field, msg: "must be greater than 0"}
	}
	return nil
}

func validatePositiveInt64(field string, value int64) error {
	if value <= 0 {
		return validationError{field: field, msg: "must be greater than 0"}
	}
	return nil
}

func validatePort(port int) error {
	if port <= 0 || port > 65535 {
		return validationError{
			field: "server.port",
			msg:   fmt.Sprintf("must be between 1 and 65535, got %d", port),
		}
	}
	return nil
}
