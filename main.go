package main

import (
	"context"
	"os"

	"github.com/moby/buildkit/client/llb"
)

func main() {
	def := llb.Image("docker.io/library/alpine:latest").
		Run(llb.Shlex(`sh -c "sleep 10 && echo -n test > /test"`))

	dt, err := def.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		panic(err)
	}
	llb.WriteTo(dt, os.Stdout)
}
