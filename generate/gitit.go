package generate

import (
	"os/exec"
	"path/filepath"
)

func git_add(gcmd *generate, gomod *goMod, podPath, localPkgPart string) (string, error) {
	if len(gomod.dirty) == 0 {
		return "", nil
	}
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("go")
	cmd.Dir = outAbs
	args := []string{
		"add",
	}
	args = append(args, gomod.dirty...)
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

func git_commit(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	commitMsg string,
) (string, error) {
	if len(gomod.dirty) == 0 {
		return "", nil
	}
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("go")
	cmd.Dir = outAbs
	args := []string{
		"commit",
		"-m", commitMsg,
	}
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

func git_tag(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	commitMsg string,
) (string, error) {
	if len(gomod.dirty) == 0 {
		return "", nil
	}
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("go")
	cmd.Dir = outAbs
	base := ""
	if podPath == "." {
		base = localPkgPart
	} else {
		base = podPath + "/" + localPkgPart
	}

	args := []string{
		"tag",
		"-a",
		"-m", commitMsg,
		base + "@" + gomod.version,
	}
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}
