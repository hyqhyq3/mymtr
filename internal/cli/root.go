package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/hyqhyq3/mymtr/internal/geoip"
	"github.com/hyqhyq3/mymtr/internal/mtr"
	"github.com/hyqhyq3/mymtr/internal/tui"
)

type rootOptions struct {
	maxHops   int
	count     int
	interval  time.Duration
	timeout   time.Duration
	protocol  string
	ipVersion int
	noDNS     bool
	geoip     string
	ip2rDB    string
	ip2rURL   string
	noGeoIP   bool
	json      bool
	tui       bool
	noTUI     bool
	autoDLGeo bool
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{tui: true}

	cmd := &cobra.Command{
		Use:           "mymtr <target>",
		Short:         "带 IP 地理位置解析的网络诊断工具（MTR 风格）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			useTUI := opts.tui && !opts.noTUI && !opts.json

			count := opts.count
			if useTUI && count == 10 && !cmd.Flags().Changed("count") {
				// TUI 默认更适合无限探测；如需有限轮数可显式指定 --count
				count = 0
			}
			if count == 0 && !useTUI {
				count = 1
			}
			cfg := &mtr.Config{
				Target:    target,
				MaxHops:   opts.maxHops,
				Count:     count,
				Interval:  opts.interval,
				Timeout:   opts.timeout,
				Protocol:  mtr.Protocol(opts.protocol),
				IPVersion: opts.ipVersion,
				EnableDNS: !opts.noDNS,
			}

			prober, err := mtr.NewProber(cfg.Protocol, cfg.IPVersion, cfg.Timeout)
			if err != nil {
				return err
			}
			defer prober.Close()

			geoipSource := opts.geoip
			if opts.noGeoIP {
				geoipSource = "off"
			}
			resolver, err := geoip.NewResolver(geoipSource, geoip.Options{
				IP2RegionDB:  opts.ip2rDB,
				IP2RegionURL: opts.ip2rURL,
				AutoDownload: opts.autoDLGeo,
			})
			if err != nil {
				return err
			}
			defer resolver.Close()

			controller, err := mtr.NewController(cfg, prober, resolver)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if useTUI {
				ctx, cancel := context.WithCancel(ctx)
				errCh := make(chan error, 1)
				go func() { errCh <- controller.Run(ctx) }()

				if err := tui.Run(ctx, cancel, controller); err != nil {
					cancel()
					return err
				}

				cancel()
				select {
				case err = <-errCh:
					if err != nil && !errors.Is(err, context.Canceled) {
						return err
					}
					return nil
				case <-time.After(300 * time.Millisecond):
					// 不阻塞退出：defer 会关闭 prober/resolver，Probe 会被打断并退出。
					return nil
				}
			}

			if err := controller.Run(ctx); err != nil {
				return err
			}

			snapshot := controller.Snapshot()
			if opts.json {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(snapshot)
			}

			return renderText(snapshot)
		},
	}

	cmd.Flags().IntVar(&opts.maxHops, "max-hops", 30, "最大跳数")
	cmd.Flags().IntVar(&opts.count, "count", 10, "探测轮数（0=无限，CLI 模式建议设置为正数）")
	cmd.Flags().DurationVar(&opts.interval, "interval", time.Second, "每轮探测间隔")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", time.Second, "单次探测超时")
	cmd.Flags().StringVar(&opts.protocol, "protocol", string(mtr.ProtocolICMP), "探测协议：icmp/udp")
	cmd.Flags().IntVar(&opts.ipVersion, "ip-version", 4, "IP 版本：4/6")
	cmd.Flags().BoolVar(&opts.noDNS, "no-dns", false, "禁用反向 DNS")
	cmd.Flags().StringVar(&opts.geoip, "geoip", "cip", "IP 地理位置数据源：cip/ip2region/off")
	cmd.Flags().StringVar(&opts.ip2rDB, "ip2region-db", "data/ip2region.xdb", "ip2region 数据库路径（当 --geoip=ip2region 时使用）")
	cmd.Flags().StringVar(&opts.ip2rURL, "geoip-ip2region-url", "", "自定义 ip2region 数据库下载地址（默认自动选择官方源）")
	cmd.Flags().BoolVar(&opts.autoDLGeo, "geoip-auto-download", true, "当使用 ip2region 且数据库缺失时自动下载")
	cmd.Flags().BoolVar(&opts.noGeoIP, "no-geoip", false, "禁用 IP 地理位置解析")
	cmd.Flags().BoolVar(&opts.json, "json", false, "输出 JSON")
	cmd.Flags().BoolVar(&opts.tui, "tui", true, "启用 TUI 实时界面（默认开启）")
	cmd.Flags().BoolVar(&opts.noTUI, "no-tui", false, "禁用 TUI，使用一次性输出模式")

	return cmd
}

func renderText(s *mtr.Snapshot) error {
	if s == nil {
		return errors.New("空结果")
	}

	fmt.Printf("Target: %s (%s)  Protocol: %s  Rounds: %d\n\n", s.Target, s.TargetIP, s.Protocol, s.Count)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TTL\tLoss%\tSnt\tRcv\tLast\tAvg\tBest\tWrst\tStDev\tAddress\tHostname\tLocation")
	for _, hop := range s.Hops {
		address := "*"
		if hop.IP != "" {
			address = hop.IP
		}
		hostname := hop.Hostname
		if strings.TrimSpace(hostname) == "" {
			hostname = "-"
		}
		location := ""
		if hop.Location != nil {
			location = hop.Location.String()
		}
		if strings.TrimSpace(location) == "" {
			location = "-"
		}

		stats := hop.Stats
		fmt.Fprintf(
			w,
			"%d\t%.1f\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			hop.TTL,
			stats.Loss,
			stats.Sent,
			stats.Received,
			emptyAsDash(stats.Last),
			emptyAsDash(stats.Avg),
			emptyAsDash(stats.Best),
			emptyAsDash(stats.Worst),
			emptyAsDash(stats.StdDev),
			address,
			hostname,
			location,
		)
	}
	return w.Flush()
}

func emptyAsDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
