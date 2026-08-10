package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// createNexArchive archives srcDir into destPath as a .nex package
// (gzip-compressed tar with a .nex extension).
func createNexArchive(srcDir, destPath string) error {
	return createTarGz(srcDir, destPath)
}

// createTarGz archives srcDir into destPath as a gzip-compressed tar bundle.
// It skips .modules, .git, and the destination archive itself.
func createTarGz(srcDir, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()

	gzw := gzip.NewWriter(out)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	absDest, _ := filepath.Abs(destPath)
	srcDir = filepath.Clean(srcDir)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		// Skip common non-source directories and the archive being written.
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) > 0 {
			switch parts[0] {
			case ".git", ".modules", ".nex":
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		absPath, _ := filepath.Abs(path)
		if absPath == absDest {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("tar header for %s: %w", path, err)
		}
		header.Name = filepath.ToSlash(rel)

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header for %s: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("write %s to archive: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// sha256File returns the hex-encoded SHA-256 checksum of path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractNexArchive unpacks a .nex (or .tar.gz) package into destDir.
func extractNexArchive(archivePath, destDir string) error {
	return extractTarGz(archivePath, destDir)
}

// extractTarGz unpacks a gzip-compressed tar archive into destDir.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	destDir = filepath.Clean(destDir)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		target, err := safeJoin(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			mode := os.FileMode(header.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("extract %s: %w", target, err)
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			// Skip symlinks and other special entries for safety.
		}
	}
	return nil
}

// safeJoin joins base and name, rejecting path traversal.
func safeJoin(base, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid archive path (traversal): %s", name)
	}
	target := filepath.Join(base, cleaned)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid archive path (escapes destination): %s", name)
	}
	return target, nil
}
