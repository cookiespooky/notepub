package localutil

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cookiespooky/notepub/internal/s3util"
)

func ListMarkdown(root, prefix string) ([]s3util.Object, error) {
	if root == "" {
		return nil, fmt.Errorf("local content root is empty")
	}
	root = filepath.Clean(root)
	listRoot := root
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if prefix != "" {
		listRoot = filepath.Join(root, filepath.FromSlash(prefix))
	}
	if _, err := os.Stat(listRoot); err != nil {
		return nil, err
	}

	out := []s3util.Object{}
	err := filepath.WalkDir(listRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(listRoot, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if key == "." || key == "" {
			return nil
		}
		if prefix != "" {
			key = path.Join(prefix, key)
		}
		etag, err := fileETag(p)
		if err != nil {
			return err
		}
		mod := info.ModTime().UTC()
		out = append(out, s3util.Object{
			Key:          key,
			ETag:         etag,
			LastModified: ptrTime(mod),
			Size:         info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func FetchObject(root, key string) ([]byte, error) {
	path, err := ResolvePath(root, key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func ResolvePath(root, key string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("local content root is empty")
	}
	if key == "" {
		return "", fmt.Errorf("local content key is empty")
	}
	clean := path.Clean("/" + key)
	if strings.Contains(clean, "..") || clean == "/" {
		return "", fmt.Errorf("invalid local key: %q", key)
	}
	rel := strings.TrimPrefix(clean, "/")
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(rel))
	full = filepath.Clean(full)
	if full != rootAbs && !strings.HasPrefix(full, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid local key: %q", key)
	}
	return full, nil
}

func fileETag(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
