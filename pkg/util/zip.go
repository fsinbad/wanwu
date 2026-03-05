package util

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDir 将目录打包为 zip 格式数据。
// srcDir: 源目录路径，支持两种模式：
//   - "/path/to/dir"：包含最后一级目录名，zip 内容为 "dir/file1.txt"
//   - "/path/to/dir/."：不包含最后一级目录名，zip 内容为 "file1.txt"
func ZipDir(srcDir string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	skipBase := strings.HasSuffix(srcDir, string(os.PathSeparator)+".")
	srcDir = filepath.Clean(srcDir)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory not found: %s", srcDir)
	}

	var baseName string
	if skipBase {
		baseName = ""
	} else {
		baseName = filepath.Base(srcDir)
	}

	err := filepath.Walk(srcDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, filePath)
		if err != nil {
			return fmt.Errorf("get relative path failed: %w", err)
		}

		var zipPath string
		if relPath == "." {
			if skipBase {
				return nil
			}
			zipPath = baseName + "/"
		} else {
			if skipBase {
				zipPath = filepath.ToSlash(relPath)
			} else {
				zipPath = filepath.ToSlash(filepath.Join(baseName, relPath))
			}
		}

		if info.IsDir() {
			zipPath += "/"
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("create zip header failed: %w", err)
		}
		header.Name = zipPath
		header.Method = zip.Store

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create zip entry failed: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file failed: %w", err)
		}
		_, err = io.Copy(writer, file)
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("write file content failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory failed: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer failed: %w", err)
	}

	return buf.Bytes(), nil
}
