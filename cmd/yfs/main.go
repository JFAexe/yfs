package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/erikgeiser/ctxio"
	"github.com/goccy/go-yaml"
	"github.com/urfave/cli/v3"

	"github.com/JFAexe/tem/pkg/staticfs"
)

var (
	version = "custom"
	commit  = "unknown"
	date    = "unknown date"
)

var app = &cli.Command{
	Name:    "yfs",
	Usage:   "tiny go yaml file server",
	Version: fmt.Sprintf("%s (%s) built using %s on %s", version, commit, runtime.Version(), date),
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Sources: cli.EnvVars("YFS_INPUT_FILE"),
			Value:   "-",
			Usage:   "input file path\vreads from stdin if not specified or set to '-'\r",
			Config: cli.StringConfig{
				TrimSpace: true,
			},
		},
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Sources: cli.EnvVars("YFS_ADDRESS"),
			Value:   "0.0.0.0",
			Usage:   "address used by the server\vsupports IPv4 and IPv6 format\r",
			Config: cli.StringConfig{
				TrimSpace: true,
			},
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Sources: cli.EnvVars("YFS_PORT"),
			Usage:   "port used by the server\vselects a random port if not specified or set to 0\r",
		},
	},
	Metadata: map[string]any{
		"name": os.Args[0],
		"notes": []string{
			"The server does not provide custom base paths, auth, or compression",
			"Supports multiple yaml documents in a single input",
			"Accepts an array of objects with `path` and `data` keys",
			"Text values can be raw text or base64 with `!!binary` tag",
		},
	},
	Action: run,
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flagger := cli.FlagStringer

	cli.FlagStringer = func(flag cli.Flag) string {
		return strings.NewReplacer(
			"\t", "\n\n\t",
			"\v", "\n\n\t",
			"\r ", "\n\t\n\t",
		).Replace(flagger(flag))
	}

	cli.RootCommandHelpTemplate = strings.Join([]string{
		"\n {{ .Name }} - {{ .Usage }}",
		"Usage: {{ index .Metadata `name` }} [flags]",
		"Flags:\n{{- range .VisibleFlags }}\n{{ .String | nindent 3 }}{{- end }}",
		"Notes:\n{{- range index .Metadata `notes` }}\n {{ . | nindent 3 }}{{- end }}",
		"Version: {{ .Version }}\n\n",
	}, "\n\n ")

	if err := app.Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	var (
		inputPath     = cmd.String("input")
		listenAddress = cmd.String("address")
		listenPort    = cmd.Int("port")

		inputFile = os.Stdin
	)

	if inputPath = strings.TrimSpace(inputPath); inputPath != "" && inputPath != "-" {
		abs, err := filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("failed to get abs path for input file: %w", err)
		}

		if inputFile, err = os.Open(abs); err != nil {
			return fmt.Errorf("failed to open input file: %w", err)
		}
		defer inputFile.Close() //nolint:errcheck
	}

	sfs, err := initFS(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to initialize fs: %w", err)
	}

	lnr, err := net.Listen("tcp", net.JoinHostPort(listenAddress, strconv.Itoa(max(0, listenPort))))
	if err != nil {
		return fmt.Errorf("failed to listen on specified address: %w", err)
	}
	defer lnr.Close() //nolint:errcheck

	var (
		ech = make(chan error, 1)
		srv = &http.Server{
			Handler: http.FileServerFS(sfs),
		}
	)

	defer close(ech)

	go func() {
		if err := srv.Serve(lnr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ech <- fmt.Errorf("failed to serve: %w", err)
		}
	}()

	_, port, _ := net.SplitHostPort(lnr.Addr().String())

	fmt.Fprintln(os.Stdout, &url.URL{Scheme: "http", Host: net.JoinHostPort(listenAddress, port)}) //nolint:errcheck

	select {
	case err := <-ech:
		return err
	case <-ctx.Done():
		scx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		if err := srv.Shutdown(scx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}

		return nil
	}
}

func initFS(ctx context.Context, file *os.File) (*staticfs.FS, error) {
	rcx, cancel := context.WithCancel(ctx)
	defer cancel()

	r, err := wrapFile(rcx, file)
	if err != nil {
		return nil, fmt.Errorf("failed to create env file reader: %w", err)
	}

	var (
		files []staticfs.File

		dec = yaml.NewDecoder(r, yaml.CustomUnmarshalerContext(func(ctx context.Context, d *[]byte, r []byte) error {
			var s string

			if err := yaml.UnmarshalContext(ctx, r, &s); err != nil {
				return err
			}

			*d = []byte(s)

			return nil
		}))
	)

	for {
		var temp []struct {
			Path string `yaml:"path"`
			Data []byte `yaml:"data"`
		}

		if err := dec.DecodeContext(rcx, &temp); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("failed to decode raw file: %w", err)
		}

		for _, f := range temp {
			files = append(files, staticfs.File{
				Path: f.Path,
				Data: f.Data,
			})
		}
	}

	return staticfs.New(files, staticfs.WithOverwrite(true))
}

func wrapFile(ctx context.Context, file *os.File) (io.ReadCloser, error) {
	if file == nil {
		return nil, fmt.Errorf("nil file")
	}

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		r, err := ctxio.WrapFile(file)
		if err != nil {
			return nil, err
		}

		go func() { <-ctx.Done(); r.Close() }() //nolint:errcheck

		return r, nil
	}

	return file, nil
}
