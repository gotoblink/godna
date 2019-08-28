package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wxio/godna/pb/dna/config"
)

func (proc *Step1) Process(rootOutDir string, cfg *config.Config) (string, error) {
	if err := proc.collectGomods(cfg); err != nil {
		return "", err
	}
	return "", nil
}

func step1(cfg *config.Config) (*Step1, error) {
	gomods := &Step1{}
	if err := gomods.collectGomods(cfg); err != nil {
		return nil, err
	}
	return gomods, nil
}

var goModRe = regexp.MustCompile(`(?m)^module ([^ ]+)$`)

func (in *Step1) collectGomods(cfg *config.Config) error {
	walkCollectGoMods := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || filepath.Base(path) != "go.mod" {
			return nil
		}
		gm := goMod{}
		if rel, err := filepath.Rel(cfg.SrcDir, filepath.Dir(path)); err != nil {
			return err
		} else {
			gm.RelDir = rel
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if match := goModRe.FindSubmatch(content); len(match) > 0 {
			gm.Module = strings.TrimSpace(string(match[1]))
		} else {
			return fmt.Errorf("no go module found in %s/go.mod path: %s\n%s\n", gm.RelDir, path, string(content))
		}
		in.Modules = append(in.Modules, gm)
		return nil
	}
	if err := filepath.Walk(cfg.SrcDir, walkCollectGoMods); err != nil {
		return err
	}
	return nil
}
