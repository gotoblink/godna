package regen

import (
	"os"
	"path/filepath"

	"github.com/wxio/godna/config"
)

type Regen struct {
	cfg       *config.Config
	OutputDir string `opts:"mode=arg" help:"output directory eg ."`
}

func New(cfg *config.Config) *Regen {
	return &Regen{cfg: cfg}
}

func (cmd *Regen) Run() error {
	var err error
	cmd.cfg.SrcDir = os.ExpandEnv(cmd.cfg.SrcDir)
	cmd.cfg.SrcDir, err = filepath.Abs(cmd.cfg.SrcDir)
	if err != nil {
		return err
	}
	cmd.OutputDir = os.ExpandEnv(cmd.OutputDir)
	cmd.OutputDir, err = filepath.Abs(cmd.OutputDir)
	if err != nil {
		return err
	}
	//
	gomods := &Step1{}
	if _, err = gomods.Process(cmd.OutputDir, cmd.cfg); err != nil {
		return err
	}
	//
	gomods2 := &Step2{step1: *gomods}
	if _, err = gomods2.Process(cmd.OutputDir, cmd.cfg); err != nil {
		return err
	}
	pkgs := &Step3{step2: *gomods2}
	if _, err = pkgs.Process(cmd.OutputDir, cmd.cfg); err != nil {
		return err
	}
	st4 := &Step4{step3: *pkgs}
	if _, err = st4.Process(cmd.OutputDir, cmd.cfg); err != nil {
		return err
	}
	return nil
}
