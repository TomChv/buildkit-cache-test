package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	bk "github.com/moby/buildkit/client"
	_ "github.com/moby/buildkit/client/connhelper/dockercontainer" // import the container connection driver
	bkgw "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"golang.org/x/sync/errgroup"

	"github.com/moby/buildkit/client/llb"
)

func generate() {
	def := llb.Image("docker.io/library/alpine:latest").
		Run(llb.Shlex(`sh -c "sleep 10 && echo -n test > /test"`))

	dt, err := def.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		panic(err)
	}
	llb.WriteTo(dt, os.Stdout)
}

type Client struct {
	c *bk.Client
}

func New(ctx context.Context) (*Client, error) {
	c, err := bk.New(ctx, os.Getenv("BUILDKIT_HOST"))
	if err != nil {
		return nil, err
	}

	return &Client{c}, nil
}

func (c *Client) DoWrapped(ctx context.Context) error {
	attrs := map[string]string{
		"scope": "test-cache-wrapped",
	}

	attrs["url"] = os.Getenv("ACTIONS_CACHE_URL")
	attrs["token"] = os.Getenv("ACTIONS_RUNTIME_TOKEN")

	opts := bk.SolveOpt{
		Exports: []bk.ExportEntry{{Type: "local", OutputDir: "result"}},
		CacheImports: []bk.CacheOptionsEntry{{
			Type:  "gha",
			Attrs: attrs,
		}},
		CacheExports: []bk.CacheOptionsEntry{{
			Type:  "gha",
			Attrs: attrs,
		}},
		Session: []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)},
	}

	fmt.Printf("Cache imports: %v\n", opts.CacheImports)
	fmt.Printf("Cache exports: %v\n", opts.CacheExports)

	return c.execWrapped(ctx, opts)
}

func (c *Client) execWrapped(ctx context.Context, opts bk.SolveOpt) error {
	eg, ctx := errgroup.WithContext(ctx)

	attrs := map[string]string{
		"scope": "test-cache-wrapped",
	}

	attrs["url"] = os.Getenv("ACTIONS_CACHE_URL")
	attrs["token"] = os.Getenv("ACTIONS_RUNTIME_TOKEN")

	eg.Go(func() error {
		wg := sync.WaitGroup{}
		defer func() {
			wg.Wait()
		}()

		status := make(chan *bk.SolveStatus)
		wg.Add(1)

		// Catch channel
		go func() {
			for _ = range status {
			}
			wg.Done()
		}()

		_, err := c.c.Build(ctx, opts, "",
			func(ctx context.Context, c bkgw.Client) (*bkgw.Result, error) {
				state := llb.
					Image("alpine").
					User("root").
					Run(llb.Shlex(`sh -c "sleep 10 && echo -n test > /test"`))

				def, err := state.Marshal(ctx, llb.LinuxArm64)
				if err != nil {
					return nil, err
				}

				res, err := c.Solve(ctx, bkgw.SolveRequest{
					Definition: def.ToPB(),
					CacheImports: []bkgw.CacheOptionsEntry{{
						Type:  "gha",
						Attrs: attrs,
					}},
					//	CacheImports: []bkgw.CacheOptionsEntry{{
					//		Type: "local",
					//		Attrs: map[string]string{
					//			"src": "store",
					//		},
					//	}},
				})
				if err != nil {
					return nil, err
				}
				return res, nil
			}, status)
		return err
	})

	return eg.Wait()
}

func buildWrapped() {
	ctx := context.Background()
	c, err := New(ctx)
	if err != nil {
		panic(err)
	}

	n := time.Now()
	err = c.DoWrapped(ctx)
	fmt.Printf("Done in %v\n", time.Since(n))
	if err != nil {
		fmt.Println(err)
	}
}

func (c *Client) DoSimple(ctx context.Context) error {
	attrs := map[string]string{
		"scope": "test-cache-simple",
	}

	attrs["url"] = os.Getenv("ACTIONS_CACHE_URL")
	attrs["token"] = os.Getenv("ACTIONS_RUNTIME_TOKEN")

	opts := bk.SolveOpt{
		Exports: []bk.ExportEntry{{Type: "local", OutputDir: "result"}},
		CacheImports: []bk.CacheOptionsEntry{{
			Type:  "gha",
			Attrs: attrs,
		}},
		CacheExports: []bk.CacheOptionsEntry{{
			Type:  "gha",
			Attrs: attrs,
		}},
		Session: []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)},
	}

	fmt.Printf("Cache imports: %v\n", opts.CacheImports)
	fmt.Printf("Cache exports: %v\n", opts.CacheExports)

	return c.execSimple(ctx, opts)
}

func (c *Client) execSimple(ctx context.Context, opts bk.SolveOpt) error {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		wg := sync.WaitGroup{}
		defer func() {
			wg.Wait()
		}()

		status := make(chan *bk.SolveStatus)
		wg.Add(1)

		// Catch channel
		go func() {
			for _ = range status {
			}
			wg.Done()
		}()

		state := llb.
			Image("alpine").
			User("root").
			Run(llb.Shlex(`sh -c "sleep 10 && echo -n test > /test"`))

		def, err := state.Marshal(ctx, llb.LinuxArm64)
		if err != nil {
			return err
		}

		_, err = c.c.Solve(ctx, def, opts, status)
		if err != nil {
			return err
		}
		return nil
	})

	return eg.Wait()
}

func buildSimple() {
	ctx := context.Background()
	c, err := New(ctx)
	if err != nil {
		panic(err)
	}

	n := time.Now()
	err = c.DoSimple(ctx)
	fmt.Printf("Done in %v\n", time.Since(n))
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) != 1 {
		log.Fatalln("Missing argument : generate or build")
	}

	if args[0] == "generate" {
		generate()
		return
	}

	if args[0] == "build-wrapped" {
		buildWrapped()
		return
	}

	if args[0] == "build-simple" {
		buildSimple()
		return
	}

	log.Fatalln("Unknown arguments")
}
