package wrapper

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const marker = "# agentenv generated wrapper v1"

func ValidateAgentName(agent string) error {
	if agent == "" || strings.ContainsAny(agent, "/\\;&|$><'\"(){}[]!*?~` \t\n") {
		return fmt.Errorf("unsafe agent name %q", agent)
	}
	return nil
}

func Install(binDir, agentenvPath, agent string) (string, error) {
	if err := ValidateAgentName(agent); err != nil {
		return "", err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(binDir, agent)
	content := fmt.Sprintf("#!/bin/sh\n%s\nexec %q run %q \"$@\"\n", marker, agentenvPath, agent)
	if existing, err := os.ReadFile(target); err == nil {
		if !strings.Contains(string(existing), marker) {
			return "", fmt.Errorf("refusing to overwrite non-agentenv file: %s", target)
		}
	}
	if err := os.WriteFile(target, []byte(content), 0o755); err != nil {
		return "", err
	}
	return target, nil
}

func List(binDir string) ([]string, error) {
	entries, err := os.ReadDir(binDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	agents := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if ValidateAgentName(entry.Name()) != nil {
			continue
		}
		path := filepath.Join(binDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), marker) {
			agents = append(agents, entry.Name())
		}
	}
	sort.Strings(agents)
	return agents, nil
}

func Uninstall(binDir, agent string) (string, error) {
	if err := ValidateAgentName(agent); err != nil {
		return "", err
	}
	target := filepath.Join(binDir, agent)
	content, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(content), marker) {
		return "", fmt.Errorf("refusing to delete non-agentenv file: %s", target)
	}
	if err := os.Remove(target); err != nil {
		return "", err
	}
	return target, nil
}
