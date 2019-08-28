package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func step1(cfg *Config) (*goMods, error) {
	gomods := &goMods{}
	if err := gomods.collectGomods(cfg); err != nil {
		return nil, err
	}
	return gomods, nil
}

var goModRe = regexp.MustCompile(`(?m)^module ([^ ]+)$`)

func (in *goMods) collectGomods(cfg *Config) error {
	walkCollectGoMods := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || filepath.Base(path) != "go.mod" {
			return nil
		}
		gm := goMod{}
		if rel, err := filepath.Rel(cfg.cfg.SrcDir, filepath.Dir(path)); err != nil {
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
	if err := filepath.Walk(cfg.cfg.SrcDir, walkCollectGoMods); err != nil {
		return err
	}
	return nil
}
