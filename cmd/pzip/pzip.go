package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/zdz1715/pzip/flate"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	gopkgversion "github.com/zdz1715/go-pkg-version"
	"github.com/zdz1715/pzip"
)

type Options struct {
	Recursive     bool
	Excludes      []string
	Includes      []string
	Quiet         bool
	Concurrency   int
	Comment       string
	NoDereference bool
	Level         int
}

func (o *Options) addFlags(flags *pflag.FlagSet) {
	flags.IntVar(&o.Concurrency, "concurrency", runtime.GOMAXPROCS(0), "设置压缩的并发数，默认为 CPU 核心数")
	flags.IntVar(&o.Level, "level", -1, "指定压缩级别，范围 0-9")
	flags.BoolVarP(&o.Recursive, "recursive", "r", true, "递归压缩目录中的文件")
	flags.BoolVarP(&o.Quiet, "quiet", "q", false, "启用静默模式，不输出日志信息（但仍显示错误信息）")
	flags.BoolVarP(&o.NoDereference, "no-dereference", "y", false, "将符号链接存储为链接，而不是链接指向的文件。")
	flags.StringSliceVarP(&o.Excludes, "exclude", "x", o.Excludes, "排除匹配的文件，支持多个排除规则，如：-x '*.log'，-x '*.tmp'")
	flags.StringSliceVarP(&o.Includes, "include", "i", o.Includes, "仅包含匹配的文件，支持多个包含规则，如：-i '*.yaml' -i 'README.md'")
	flags.StringVarP(&o.Comment, "comment", "z", "", "为整个 ZIP 文件添加注释")
}

func NewPzipCommand(ctx context.Context) *cobra.Command {
	ver := gopkgversion.NewVersionInfo()
	opts := &Options{}
	cmd := &cobra.Command{
		Use:           "pzip [flags] file[.zip] [file...]",
		Short:         "并发压缩文件至zip格式",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s %s", ver.Version, ver.Platform),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return cmd.Help()
			}
			name := pzip.FormatName(args[0])
			if len(args) < 2 {
				return fmt.Errorf("nothing to do! (%s)", name)
			}
			err := RunZip(ctx, opts, name, args[1:])
			if err != nil {
				return fmt.Errorf("%s (%s)", err, name)
			}
			return nil
		},
	}
	opts.addFlags(cmd.Flags())
	return cmd
}

func RunZip(ctx context.Context, opts *Options, name string, paths []string) error {
	after := func(hdr *pzip.FileHeader) {
		md := "stored"
		if hdr.Method == zip.Deflate {
			md = "deflated"
		}
		_, _ = fmt.Printf("  adding: %s (%s)\n", hdr.Name, md)
	}

	if opts.Quiet {
		after = nil
	}

	return pzip.Archive(ctx, name, &pzip.ArchiveOptions{
		NewCompressor: func(w io.Writer, level int) (flate.Writer, error) {
			return flate.NewFastWriter(w, level)
		},
		Concurrency: opts.Concurrency,
		Files:       paths,
		SkipPath: pzip.SkipPath{
			Includes: opts.Includes,
			Excludes: opts.Excludes,
		},
		After:       after,
		Dereference: !opts.NoDereference,
		Level:       opts.Level,
		Comment:     opts.Comment,
		Recurse:     opts.Recursive,
	})
}

func main() {
	ctx := pzip.SetupSignalContext()
	if err := NewPzipCommand(ctx).Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pzip error: %s\n", err)
		os.Exit(1)
	}
}
