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
	"github.com/hyqhyq3/mymtr/internal/i18n"
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
		Short:         i18n.T("cmd.short"),
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

	cmd.Flags().IntVar(&opts.maxHops, "max-hops", 30, i18n.T("cmd.flag.maxHops"))
	cmd.Flags().IntVar(&opts.count, "count", 10, i18n.T("cmd.flag.count"))
	cmd.Flags().DurationVar(&opts.interval, "interval", time.Second, i18n.T("cmd.flag.interval"))
	cmd.Flags().DurationVar(&opts.timeout, "timeout", time.Second, i18n.T("cmd.flag.timeout"))
	cmd.Flags().StringVar(&opts.protocol, "protocol", string(mtr.ProtocolICMP), i18n.T("cmd.flag.protocol"))
	cmd.Flags().IntVar(&opts.ipVersion, "ip-version", 4, i18n.T("cmd.flag.ipVersion"))
	cmd.Flags().BoolVar(&opts.noDNS, "no-dns", false, i18n.T("cmd.flag.noDNS"))
	cmd.Flags().StringVar(&opts.geoip, "geoip", "cip", i18n.T("cmd.flag.geoip"))
	cmd.Flags().StringVar(&opts.ip2rDB, "ip2region-db", "data/ip2region.xdb", i18n.T("cmd.flag.ip2regionDB"))
	cmd.Flags().StringVar(&opts.ip2rURL, "geoip-ip2region-url", "", i18n.T("cmd.flag.ip2regionURL"))
	cmd.Flags().BoolVar(&opts.autoDLGeo, "geoip-auto-download", true, i18n.T("cmd.flag.autoDLGeo"))
	cmd.Flags().BoolVar(&opts.noGeoIP, "no-geoip", false, i18n.T("cmd.flag.noGeoIP"))
	cmd.Flags().BoolVar(&opts.json, "json", false, i18n.T("cmd.flag.json"))
	cmd.Flags().BoolVar(&opts.tui, "tui", true, i18n.T("cmd.flag.tui"))
	cmd.Flags().BoolVar(&opts.noTUI, "no-tui", false, i18n.T("cmd.flag.noTUI"))

	return cmd
}

func renderText(s *mtr.Snapshot) error {
	if s == nil {
		return errors.New(i18n.T("err.emptyResult"))
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
