package readfds

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	_ "github.com/wxio/godna/pb/extensions/store"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type readfds struct {
}

func New() *readfds {
	return &readfds{}
}

func (cmd *readfds) Run() error {
	fds := &descriptor.FileDescriptorSet{}
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(buf, fds)
	if err != nil {
		return err
	}
	// fileDescriptorProto
	for _, fdp := range fds.File {
		options := fmt.Sprintf("%v\n", fdp.Options)
		if strings.Contains(options, "[wxio.dna.store]:<go_mod:true >") {
			fmt.Printf("%s\n", *fdp.Options.GoPackage)
		}
	}
	return nil
}
