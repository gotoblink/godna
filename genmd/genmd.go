package genmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/jpillora/md-tmpl/mdtmpl"
)

type genmd struct {
	//
	Filename   string `opts:"mode=arg"`
	WorkingDir string
	Preview    bool
	StdOut     bool
}

func New() *genmd {
	gen := &genmd{
		WorkingDir: ".",
	}
	return gen
}

func (gen *genmd) Run() error {
	fp := filepath.Join(gen.WorkingDir, gen.Filename)
	if b, err := ioutil.ReadFile(fp); err != nil {
		return err
	} else {
		if gen.Preview {
			for i, c := range mdtmpl.Commands(b) {
				fmt.Printf("%18s#%d %s\n", gen.Filename, i+1, c)
			}
			return nil
		}
		b = mdtmpl.ExecuteIn(b, filepath.Join(gen.WorkingDir))
		if gen.StdOut {
			fmt.Printf("\n%s\n", string(b))
		} else {
			if err := ioutil.WriteFile(fp, b, 0655); err != nil {
				return err
			}
			log.Printf("executed templates and rewrote '%s'", gen.Filename)
		}
		return nil
	}
}
