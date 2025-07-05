package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

type MusicFile struct {
	Path     string
	Name     string
	Dir      string
	Size     int64
	Modified int64
}

var audioExts = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".wav":  true,
	".m4a":  true,
	".aac":  true,
	".wma":  true,
	".opus": true,
}

func ScanDirectories(dirs []string) ([]MusicFile, error) {
	var files []MusicFile

	for _, dir := range dirs {
		expanded := expandPath(dir)
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(expanded, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !audioExts[ext] {
				return nil
			}

			files = append(files, MusicFile{
				Path:     path,
				Name:     info.Name(),
				Dir:      filepath.Dir(path),
				Size:     info.Size(),
				Modified: info.ModTime().Unix(),
			})

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
