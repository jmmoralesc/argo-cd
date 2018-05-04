package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsurePrefix(t *testing.T) {
	data := [][]string{
		{"world", "hello", "helloworld"},
		{"helloworld", "hello", "helloworld"},
		{"example.com", "https://", "https://example.com"},
		{"https://example.com", "https://", "https://example.com"},
		{"cd", "argo", "argocd"},
		{"argocd", "argo", "argocd"},
		{"", "argocd", "argocd"},
		{"argocd", "", "argocd"},
	}
	for _, table := range data {
		result := ensurePrefix(table[0], table[1])
		assert.Equal(t, table[2], result)
	}
}

func TestEnsureSuffix(t *testing.T) {
	data := [][]string{
		{"hello", "world", "helloworld"},
		{"helloworld", "world", "helloworld"},
		{"repo", ".git", "repo.git"},
		{"repo.git", ".git", "repo.git"},
		{"", "repo.git", "repo.git"},
		{"argo", "cd", "argocd"},
		{"argocd", "cd", "argocd"},
		{"argocd", "", "argocd"},
		{"", "argocd", "argocd"},
	}
	for _, table := range data {
		result := ensureSuffix(table[0], table[1])
		assert.Equal(t, table[2], result)
	}
}

func TestIsSSHUrl(t *testing.T) {
	data := map[string]bool{
		"git@GITHUB.com:argoproj/test.git":       true,
		"git@github.com:test.git":                true,
		"ssh://git@GITHUB.com:argoproj/test.git": true,
		"ssh://git@github.com:test.git":          true,
		"https://github.com/argoproj/test.git":   false,
		"git://github.com/argoproj/test.git":     false,
	}
	for k, v := range data {
		assert.Equal(t, v, IsSshURL(k))
	}
}

func TestNormalizeUrl(t *testing.T) {
	data := map[string]string{
		"git@GITHUB.com:test.git":                "git@github.com:test.git",
		"https://github.com/TEST.git":            "https://github.com/TEST.git",
		"git@GITHUB.com:argoproj/test.git":       "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:argoproj/test.git": "git@github.com:argoproj/test.git",
		"https://GITHUB.com/argoproj/test.git":   "https://github.com/argoproj/test.git",
	}
	for k, v := range data {
		assert.Equal(t, v, NormalizeGitURL(k))
	}
}
