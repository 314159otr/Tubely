package main

import (
	"fmt"
	"strings"
	"path/filepath"
	"os"

	"github.com/google/uuid"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func mediaTypeToFileExtension(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getFilename(videoID uuid.UUID, mediaType string) string {
	fileExtension := mediaTypeToFileExtension(mediaType)
	return fmt.Sprintf("%s%s", videoID, fileExtension)
}

func (cfg apiConfig) getAssetDiskPath(filename string) string {
	return filepath.Join(cfg.assetsRoot, filename)
}

func (cfg apiConfig) getAssetURL(filename string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)
}

