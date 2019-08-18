package main

type Proto_ed struct {
}

// func Protoc(in config.Config, src *Src, resp *Proto_ed) error {
// 	cmd := exec.Command("protoc")
// 	args := []string{}
// 	for _, gen := range in.generators {
// 		arg := "--" + gen.name + "_out"
// 		if len(gen.params) > 0 {
// 			arg += "="
// 			for i, kv := range gen.params {
// 				if i != 0 {
// 					arg += ","
// 				}
// 				arg += kv.key + "=" + kv.value
// 			}
// 		}
// 		if gen.outdir != "" {
// 			out := filepath.Join(in.OutputDir, gen.outdir)
// 			arg += ":" + out
// 		} else {
// 			arg += ":" + in.OutputDir
// 		}
// 		args = append(args, arg)
// 	}
// 	// args := []string{"--go_out=plugins=micro,paths=source_relative:" + oAbs}
// 	cmd.Dir = in.SrcDir
// 	// args = append(args, "-I"+srcDir)
// 	for _, inc := range in.Includes {
// 		args = append(args, "-I"+inc)
// 	}
// 	args = append(args, "-I.")
// 	for _, fi := range in.packages[pkg].files {
// 		args = append(args, fi.name)
// 	}
// 	cmd.Args = append(cmd.Args, args...)
// 	// fmt.Printf("wd: %v, cmd %+v\n", src, cmd.Args)
// 	out, err := cmd.CombinedOutput()
// 	return out, fmt.Sprintf("files-%d", len(in.packages[pkg].files)), err
// 	return nil
// }
