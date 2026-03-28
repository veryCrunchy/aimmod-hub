package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

type llmManifest struct {
	Version string                      `json:"version"`
	Runtime map[string]llmRuntimeTarget `json:"runtime"`
	Model   llmModelTarget              `json:"model"`
}

type llmRuntimeTarget struct {
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	ArchiveType string `json:"archiveType"`
	Extras      []llmRuntimeTargetExtra `json:"extras,omitempty"`
}

type llmRuntimeTargetExtra struct {
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	ArchiveType string `json:"archiveType"`
}

type llmModelTarget struct {
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Filename string `json:"filename"`
}

func hasLLMManifest(cfg Config) bool {
	if strings.TrimSpace(cfg.LLMManifestVersion) == "" {
		return false
	}
	if strings.TrimSpace(cfg.LLMModelURL) == "" ||
		strings.TrimSpace(cfg.LLMModelSHA256) == "" ||
		strings.TrimSpace(cfg.LLMModelFilename) == "" {
		return false
	}
	return runtimeConfigured(
		cfg.LLMRuntimeWindowsX64URL,
		cfg.LLMRuntimeWindowsX64SHA256,
		cfg.LLMRuntimeWindowsX64ArchiveType,
	) || runtimeConfigured(
		cfg.LLMRuntimeWindowsArm64URL,
		cfg.LLMRuntimeWindowsArm64SHA256,
		cfg.LLMRuntimeWindowsArm64ArchiveType,
	)
}

func newLLMManifestHandler(cfg Config) http.Handler {
	manifest := buildLLMManifest(cfg)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(manifest)
	})
}

func buildLLMManifest(cfg Config) llmManifest {
	manifest := llmManifest{
		Version: cfg.LLMManifestVersion,
		Runtime: map[string]llmRuntimeTarget{},
		Model: llmModelTarget{
			URL:      cfg.LLMModelURL,
			SHA256:   cfg.LLMModelSHA256,
			Filename: cfg.LLMModelFilename,
		},
	}

	if runtimeConfigured(
		cfg.LLMRuntimeWindowsX64URL,
		cfg.LLMRuntimeWindowsX64SHA256,
		cfg.LLMRuntimeWindowsX64ArchiveType,
	) {
		runtime := llmRuntimeTarget{
			URL:         cfg.LLMRuntimeWindowsX64URL,
			SHA256:      cfg.LLMRuntimeWindowsX64SHA256,
			ArchiveType: cfg.LLMRuntimeWindowsX64ArchiveType,
		}
		if runtimeConfigured(
			cfg.LLMRuntimeWindowsX64ExtraURL,
			cfg.LLMRuntimeWindowsX64ExtraSHA256,
			cfg.LLMRuntimeWindowsX64ExtraArchiveType,
		) {
			runtime.Extras = append(runtime.Extras, llmRuntimeTargetExtra{
				URL:         cfg.LLMRuntimeWindowsX64ExtraURL,
				SHA256:      cfg.LLMRuntimeWindowsX64ExtraSHA256,
				ArchiveType: cfg.LLMRuntimeWindowsX64ExtraArchiveType,
			})
		}
		manifest.Runtime["windows-x64"] = runtime
	}

	if runtimeConfigured(
		cfg.LLMRuntimeWindowsArm64URL,
		cfg.LLMRuntimeWindowsArm64SHA256,
		cfg.LLMRuntimeWindowsArm64ArchiveType,
	) {
		runtime := llmRuntimeTarget{
			URL:         cfg.LLMRuntimeWindowsArm64URL,
			SHA256:      cfg.LLMRuntimeWindowsArm64SHA256,
			ArchiveType: cfg.LLMRuntimeWindowsArm64ArchiveType,
		}
		if runtimeConfigured(
			cfg.LLMRuntimeWindowsArm64ExtraURL,
			cfg.LLMRuntimeWindowsArm64ExtraSHA256,
			cfg.LLMRuntimeWindowsArm64ExtraArchiveType,
		) {
			runtime.Extras = append(runtime.Extras, llmRuntimeTargetExtra{
				URL:         cfg.LLMRuntimeWindowsArm64ExtraURL,
				SHA256:      cfg.LLMRuntimeWindowsArm64ExtraSHA256,
				ArchiveType: cfg.LLMRuntimeWindowsArm64ExtraArchiveType,
			})
		}
		manifest.Runtime["windows-arm64"] = runtime
	}

	return manifest
}

func runtimeConfigured(url, sha256, archiveType string) bool {
	return strings.TrimSpace(url) != "" &&
		strings.TrimSpace(sha256) != "" &&
		strings.TrimSpace(archiveType) != ""
}
