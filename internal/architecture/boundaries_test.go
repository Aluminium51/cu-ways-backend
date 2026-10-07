package architecture

import (
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/Aluminium51/cu-way-backend"

// TestDependencyBoundaries checks production source, including files for other
// platforms. Tests may use persistence/HTTP adapters to verify their contracts.
func TestDependencyBoundaries(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate architecture test")
	}
	root := filepath.Join(filepath.Dir(currentFile), "..", "..")
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(relative, "internal/core/domain/") {
				// Exact decimal arithmetic is the existing monetary value type.
				if importPath == "github.com/shopspring/decimal" {
					continue
				}
				pkg, err := build.Default.Import(importPath, root, build.FindOnly)
				if err != nil || !pkg.Goroot {
					t.Errorf("%s: domain import %q must be standard library or the established decimal value type", relative, importPath)
				}
				continue
			}
			forbidden := forbiddenImports(relative)
			for _, prefix := range forbidden {
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					t.Errorf("%s: import %q violates the inward dependency boundary", relative, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func forbiddenImports(path string) []string {
	infrastructure := []string{"gorm.io", "database/sql", "net/http", "github.com/gofiber/fiber", modulePath + "/internal/repositories", modulePath + "/internal/platform/database"}
	switch {
	case strings.HasPrefix(path, "internal/core/ports/"):
		return append(infrastructure, modulePath+"/internal/services", modulePath+"/internal/handlers", modulePath+"/internal/server", modulePath+"/internal/middleware", modulePath+"/internal/platform")
	case strings.HasPrefix(path, "internal/services/"):
		return append(infrastructure, modulePath+"/internal/handlers", modulePath+"/internal/server", modulePath+"/internal/middleware")
	case strings.HasPrefix(path, "internal/handlers/"):
		return []string{"gorm.io", "database/sql", modulePath + "/internal/repositories", modulePath + "/internal/platform/database"}
	case strings.HasPrefix(path, "internal/platform/"):
		return []string{modulePath + "/internal/services", modulePath + "/internal/handlers"}
	default:
		return nil
	}
}
