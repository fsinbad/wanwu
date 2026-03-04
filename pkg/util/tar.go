package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TarDir 将目录打包为 tar 格式数据。
// srcDir: 源目录路径，支持两种模式：
//   - "/path/to/dir"：包含最后一级目录名，tar 内容为 "dir/file1.txt"
//   - "/path/to/dir/."：不包含最后一级目录名，tar 内容为 "file1.txt"
//
// gzip: 是否使用 gzip 压缩。
func TarDir(srcDir string, useGzip bool) ([]byte, error) {
	var buf bytes.Buffer
	var tw *tar.Writer
	var gw *gzip.Writer

	if useGzip {
		gw = gzip.NewWriter(&buf)
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(&buf)
	}

	// 检测是否以 /. 结尾，表示不包含基础目录名
	skipBase := strings.HasSuffix(srcDir, string(os.PathSeparator)+".")
	srcDir = filepath.Clean(srcDir)

	var baseName string
	if skipBase {
		// 不包含基础目录名，文件直接在根目录
		baseName = ""
	} else {
		baseName = filepath.Base(srcDir)
	}

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("get relative path failed: %w", err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("create tar header failed: %w", err)
		}

		if relPath == "." {
			if skipBase {
				// 跳过根目录本身
				return nil
			}
			header.Name = baseName + "/"
		} else {
			if skipBase {
				header.Name = filepath.ToSlash(relPath)
			} else {
				header.Name = baseName + "/" + filepath.ToSlash(relPath)
			}
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header failed: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file failed: %w", err)
		}
		_, err = io.Copy(tw, file)
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("write file content failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory failed: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer failed: %w", err)
	}
	if gw != nil {
		if err := gw.Close(); err != nil {
			return nil, fmt.Errorf("close gzip writer failed: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// TarFile 将单个文件打包为 tar 格式数据。
// srcFile: 源文件路径，tar 内容为文件名（不含目录路径）。
// useGzip: 是否使用 gzip 压缩。
func TarFile(srcFile string, useGzip bool) ([]byte, error) {
	var buf bytes.Buffer
	var tw *tar.Writer
	var gw *gzip.Writer

	if useGzip {
		gw = gzip.NewWriter(&buf)
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(&buf)
	}

	info, err := os.Stat(srcFile)
	if err != nil {
		return nil, fmt.Errorf("stat file failed: %w", err)
	}

	file, err := os.Open(srcFile)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer func() { _ = file.Close() }()

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, fmt.Errorf("create tar header failed: %w", err)
	}
	header.Name = filepath.Base(srcFile)

	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("write tar header failed: %w", err)
	}

	if _, err := io.Copy(tw, file); err != nil {
		return nil, fmt.Errorf("write file content failed: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer failed: %w", err)
	}
	if gw != nil {
		if err := gw.Close(); err != nil {
			return nil, fmt.Errorf("close gzip writer failed: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// Untar 将 tar 数据解压到目标目录。
// tarData: tar 数据（支持 gzip 压缩或未压缩）。
// destDir: 目标目录路径。
// 注意：此函数会跳过 tar 包中的第一级目录，直接解压文件到 destDir。
// 例如：tar 内容为 "project/file1.txt"，解压后为 "destDir/file1.txt"。
func Untar(tarData []byte, destDir string) error {
	var tr *tar.Reader

	// 自动检测是否为 gzip 压缩（gzip magic number: 0x1f 0x8b）
	if len(tarData) >= 2 && tarData[0] == 0x1f && tarData[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(tarData))
		if err != nil {
			return fmt.Errorf("create gzip reader failed: %w", err)
		}
		defer func() { _ = gr.Close() }()
		tr = tar.NewReader(gr)
	} else {
		tr = tar.NewReader(bytes.NewReader(tarData))
	}

	destDir = filepath.Clean(destDir)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header failed: %w", err)
		}

		parts := strings.Split(header.Name, "/")
		if len(parts) > 1 {
			header.Name = strings.Join(parts[1:], "/")
		}
		if header.Name == "" {
			continue
		}

		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), destDir+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path: %s (traversal attempt)", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory failed: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent directory failed: %w", err)
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file failed: %w", err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				_ = file.Close()
				_ = os.Remove(target)
				return fmt.Errorf("write file content failed: %w", err)
			}
			_ = file.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent directory failed: %w", err)
			}
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("create symlink failed: %w", err)
			}
		}
	}

	return nil
}
