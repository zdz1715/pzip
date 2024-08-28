package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	gopkgversion "github.com/zdz1715/go-pkg-version"
	"github.com/zdz1715/pzip"
)

type Options struct {
	Concurrency int

	List           bool
	DisplayComment bool
	Quiet          bool
	Dir            string
	Includes       []string
	Excludes       []string
}

func (o *Options) addFlags(flags *pflag.FlagSet) {
	flags.IntVar(&o.Concurrency, "concurrency", runtime.GOMAXPROCS(0), "设置解压的并发数，默认为 CPU 核心数")
	flags.BoolVarP(&o.DisplayComment, "display-comment", "z", false, "仅显示压缩包的注释信息")
	flags.BoolVarP(&o.Quiet, "quiet", "q", false, "启用静默模式，不输出日志信息（但仍显示错误信息）")
	flags.StringVarP(&o.Dir, "dir", "d", "", "指定解压目标目录")
	flags.BoolVarP(&o.List, "list", "l", false, "列出压缩包内的文件清单")
	flags.StringSliceVarP(&o.Excludes, "exclude", "x", o.Excludes, "排除匹配的文件，支持多个排除规则，如：-x '*.log'，-x '*.tmp'")
	flags.StringSliceVarP(&o.Includes, "include", "i", o.Includes, "仅解压匹配的文件，支持多个包含规则，如：-i '*.yaml'，-i 'README.md'")
}

func NewUnzipCommand(ctx context.Context) *cobra.Command {
	ver := gopkgversion.NewVersionInfo()
	opts := &Options{}
	cmd := &cobra.Command{
		Use:           "punzip [flags] file[.zip]",
		Short:         "并发解压zip压缩包",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s %s", ver.Version, ver.Platform),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return cmd.Help()
			}
			name := pzip.FormatName(args[0])
			err := RunUnZip(ctx, opts, name)
			if err != nil {
				return fmt.Errorf("%s (%s)", err, name)
			}
			return nil
		},
	}
	opts.addFlags(cmd.Flags())
	return cmd
}

func RunUnZip(ctx context.Context, opts *Options, name string) error {
	if opts.DisplayComment {
		comment, err := pzip.GetComment(name)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(os.Stdout, comment)
		return nil
	}

	if opts.List {
		reader, err := pzip.OpenReader(name)
		if err != nil {
			return err
		}
		defer reader.Close()
		return printList(os.Stdout, name, reader)
	}

	before := func(path string, r *pzip.ReadCloser) {
		_, _ = fmt.Fprintf(os.Stdout, "Archive: %s\n", path)
		_, _ = fmt.Fprintf(os.Stdout, "Comment: %s\n", r.Comment)
	}

	after := func(f *pzip.File, target *pzip.ExtractTarget) {
		md := "extracting"
		if f.FileInfo().IsDir() {
			md = "creating"
		}

		if f.Method == zip.Deflate {
			md = "inflating"
		}

		if pzip.IsSymlink(f.Mode()) {
			md = "symlinking"
		}

		_, _ = fmt.Fprintf(os.Stdout, "  %s: %s\n", md, target)
	}

	if opts.Quiet {
		before = nil
		after = nil
	}

	return pzip.Extract(ctx, name, &pzip.ExtractOptions{
		Concurrency: opts.Concurrency,
		Before:      before,
		After:       after,
		OutDir:      opts.Dir,
		SkipPath: pzip.SkipPath{
			Includes: opts.Includes,
			Excludes: opts.Excludes,
		},
	})
}

func printList(w io.Writer, name string, r *pzip.ReadCloser) error {
	_, _ = fmt.Fprintf(w, "Archive: %s\n", name)
	_, _ = fmt.Fprintf(w, "Comment: %s\n", r.Comment)
	_, _ = fmt.Fprintf(w, "Files:\n")
	header := []string{"Length", "Method", "Size", "Cmpr", "Date", "Time", "CRC-32", "Name"}
	data := make([][]string, 0, len(header))
	var (
		totalLength uint64
		totalSize   uint64
	)

	fileCount := 0
	dirCount := 0
	var latestTime time.Time
	for _, v := range r.File {
		if v.Modified.After(latestTime) {
			latestTime = v.Modified
		}
		if strings.HasSuffix(v.Name, "/") {
			dirCount++
		} else {
			fileCount++
		}

		method := "Stored"
		if v.Method == zip.Deflate {
			method = "Defl:N"
		}
		var ratio float64
		if v.UncompressedSize64 > v.CompressedSize64 {
			ratio = math.Round(float64(v.UncompressedSize64-v.CompressedSize64) / float64(v.UncompressedSize64) * 100)
		}

		totalLength += v.UncompressedSize64
		totalSize += v.CompressedSize64
		data = append(data, []string{
			strconv.FormatInt(int64(v.UncompressedSize64), 10),
			method,
			strconv.FormatInt(int64(v.CompressedSize64), 10),
			fmt.Sprintf("%.0f%%", ratio),
			v.Modified.Local().Format("2006-01-02"),
			v.Modified.Local().Format("15:04:05"),
			fmt.Sprintf("%08x", v.CRC32),
			v.Name,
		})
	}
	var totalRatio float64
	if totalLength > totalSize {
		totalRatio = math.Round(float64(totalLength-totalSize) / float64(totalLength) * 100)
	}
	table := tablewriter.NewWriter(w)
	table.SetBorder(false)

	table.SetHeaderLine(false)
	table.SetCenterSeparator("")
	table.SetRowSeparator("")
	table.SetColumnSeparator("")
	table.SetAlignment(tablewriter.ALIGN_DEFAULT)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetFooterAlignment(tablewriter.ALIGN_LEFT)

	footer := []string{
		strconv.FormatInt(int64(totalLength), 10),
		"",
		strconv.FormatInt(int64(totalSize), 10),
		fmt.Sprintf("%.0f%%", totalRatio),
		latestTime.Local().Format("2006-01-02"),
		latestTime.Local().Format("15:04:05"),
		"",
		fmt.Sprintf("%d files, %d folders", fileCount, dirCount),
	}

	table.SetHeader(header)
	table.AppendBulk(data)
	table.Append([]string{
		strings.Repeat("-", len(footer[0])),
		"",
		strings.Repeat("-", len(footer[2])),
		strings.Repeat("-", len(footer[3])),
		strings.Repeat("-", len(footer[4])),
		strings.Repeat("-", len(footer[5])),
		"",
		strings.Repeat("-", len(footer[7])),
	})
	table.Append(footer)

	table.Render()
	return nil
}

func main() {
	ctx := pzip.SetupSignalContext()
	if err := NewUnzipCommand(ctx).Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "punzip error: %s\n", err)
		os.Exit(1)
	}
}
