package geoip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"

	"github.com/hyqhyq3/mymtr/internal/i18n"
)

const (
	ip2RegionIdleTimeout      = 30 * time.Second
	ip2RegionDefaultUserAgent = "mymtr/geoip-downloader"
	ip2RegionURLEnv           = "MYMTR_IP2REGION_URL"
)

var (
	ip2RegionDownloadSources = []string{
		"https://github.com/lionsoul2014/ip2region/releases/latest/download/ip2region.xdb",
		"https://github.com/lionsoul2014/ip2region/releases/latest/download/ip2region_v4.xdb",
		"https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb",
	}
	ip2RegionHTTPClient           = &http.Client{}
	progressOutput      io.Writer = os.Stderr
)

type DownloadAnswer int

const (
	DownloadAsk DownloadAnswer = iota
	DownloadYes
	DownloadNo
)

type DownloadPrompt func(message string) (bool, error)

type DownloadOption struct {
	Answer DownloadAnswer
	Prompt DownloadPrompt
}

// DefaultIP2RegionDBPath 返回用户缓存目录下的默认 ip2region.xdb 存放路径；若无法获取缓存目录，退回到系统临时目录。
func DefaultIP2RegionDBPath() string {
	if cacheDir, err := os.UserCacheDir(); err == nil {
		if trimmed := strings.TrimSpace(cacheDir); trimmed != "" {
			return filepath.Join(trimmed, "mymtr", "ip2region.xdb")
		}
	}
	return filepath.Join(os.TempDir(), "mymtr", "ip2region.xdb")
}

type IP2RegionResolver struct {
	dbPath   string
	searcher *xdb.Searcher
}

func decideIP2RegionDownload(opt DownloadOption, dbPath string) (bool, error) {
	switch opt.Answer {
	case DownloadYes:
		return true, nil
	case DownloadNo:
		return false, nil
	case DownloadAsk:
		if opt.Prompt == nil {
			return false, errors.New(i18n.T("geoip.ip2region.promptUnavailable"))
		}
		message := i18n.Tf("geoip.ip2region.confirmDownload", map[string]interface{}{"Path": dbPath})
		return opt.Prompt(message)
	default:
		return false, fmt.Errorf("unsupported download answer: %d", opt.Answer)
	}
}

func NewIP2RegionResolver(dbPath string, customURL string, downloadOpt DownloadOption) (*IP2RegionResolver, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, errors.New(i18n.T("geoip.ip2region.pathEmpty"))
	}

	if err := ensureIP2RegionDB(dbPath, customURL, downloadOpt); err != nil {
		return nil, err
	}

	version, err := detectIPVersion(dbPath)
	if err != nil {
		return nil, err
	}

	searcher, err := xdb.NewWithFileOnly(version, dbPath)
	if err != nil {
		return nil, err
	}
	return &IP2RegionResolver{dbPath: dbPath, searcher: searcher}, nil
}

func (r *IP2RegionResolver) Source() string { return "ip2region" }

func (r *IP2RegionResolver) Close() error {
	if r.searcher == nil {
		return nil
	}
	r.searcher.Close()
	r.searcher = nil
	return nil
}

func (r *IP2RegionResolver) Resolve(ip net.IP) *GeoLocation {
	if ip == nil || r.searcher == nil {
		return nil
	}
	// ip2region v2 仅支持 IPv4
	if ip4 := ip.To4(); ip4 == nil {
		return nil
	}

	region, err := r.searcher.SearchByStr(ip.String())
	if err != nil || strings.TrimSpace(region) == "" {
		return nil
	}

	loc := parseIP2Region(region)
	if loc == nil {
		return nil
	}
	loc.Source = r.Source()
	loc.Raw = region
	return loc
}

func ensureIP2RegionDB(dbPath string, customURL string, opt DownloadOption) error {
	info, err := os.Stat(dbPath)
	if err == nil {
		if info.IsDir() {
			return errors.New(i18n.Tf("geoip.ip2region.pathIsDir", map[string]interface{}{"Path": dbPath}))
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.New(i18n.Tf("geoip.ip2region.unavailable", map[string]interface{}{"Error": err.Error()}))
	}
	allowed, decideErr := decideIP2RegionDownload(opt, dbPath)
	if decideErr != nil {
		return decideErr
	}
	if !allowed {
		return errors.New(i18n.T("geoip.ip2region.downloadDeclined"))
	}
	if err := downloadIP2RegionDB(dbPath, customURL); err != nil {
		return errors.New(i18n.Tf("geoip.ip2region.downloadFailed", map[string]interface{}{"Error": err.Error()}))
	}
	return nil
}

func downloadIP2RegionDB(dbPath, customURL string) error {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errors.New(i18n.Tf("geoip.ip2region.mkdirFailed", map[string]interface{}{"Error": err.Error()}))
		}
	}

	tmp := dbPath + ".download"
	sources := selectIP2RegionSources(customURL)
	var errs []error

	baseCtx := context.Background()
	for _, src := range sources {
		if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}

		err := downloadFromSource(baseCtx, src, tmp, dbPath)

		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", src, err))
	}

	msg := make([]string, 0, len(errs))
	for _, e := range errs {
		msg = append(msg, e.Error())
	}
	return errors.New(i18n.Tf("geoip.ip2region.allSourcesFailed", map[string]interface{}{"Errors": strings.Join(msg, "; ")}))
}

func selectIP2RegionSources(customURL string) []string {
	if customURL = strings.TrimSpace(customURL); customURL != "" {
		return []string{customURL}
	}
	if env := strings.TrimSpace(os.Getenv(ip2RegionURLEnv)); env != "" {
		return []string{env}
	}
	return ip2RegionDownloadSources
}

func downloadFromSource(parent context.Context, src, tmp, target string) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ip2RegionDefaultUserAgent)

	resp, err := ip2RegionHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, resp.Body)
		return errors.New(i18n.Tf("geoip.ip2region.statusCode", map[string]interface{}{"Code": resp.StatusCode}))
	}

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	pr := newProgressReporter(src, resp.ContentLength, progressOutput, ip2RegionIdleTimeout)
	pr.startIdleWatch(cancel)
	reader := io.TeeReader(resp.Body, pr)

	if _, err := io.Copy(out, reader); err != nil {
		pr.finish(err)
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		pr.finish(err)
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, target); err != nil {
		pr.finish(err)
		os.Remove(tmp)
		return err
	}
	pr.finish(nil)
	return nil
}

type progressReporter struct {
	source      string
	total       int64
	current     int64
	writer      io.Writer
	start       time.Time
	lastUpdate  time.Time
	idleTimeout time.Duration
	idleReset   chan struct{}
	doneCh      chan struct{}
	doneOnce    sync.Once
}

func newProgressReporter(source string, total int64, w io.Writer, idleTimeout time.Duration) *progressReporter {
	return &progressReporter{
		source:      source,
		total:       total,
		writer:      w,
		start:       time.Now(),
		lastUpdate:  time.Time{},
		idleTimeout: idleTimeout,
		idleReset:   make(chan struct{}, 1),
		doneCh:      make(chan struct{}),
	}
}

func (p *progressReporter) startIdleWatch(cancel context.CancelFunc) {
	if p.writer == nil || p.idleTimeout <= 0 {
		return
	}
	go func() {
		timer := time.NewTimer(p.idleTimeout)
		defer timer.Stop()
		for {
			select {
			case <-p.doneCh:
				return
			case <-timer.C:
				fmt.Fprintf(p.writer, "\n%s", i18n.Tf("geoip.ip2region.timeout", map[string]interface{}{"Source": p.source, "Duration": p.idleTimeout.String()}))
				if cancel != nil {
					cancel()
				}
				return
			case <-p.idleReset:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(p.idleTimeout)
			}
		}
	}()
}

func (p *progressReporter) signalProgress() {
	select {
	case p.idleReset <- struct{}{}:
	default:
	}
}

func (p *progressReporter) Write(b []byte) (int, error) {
	if p.writer == nil {
		return len(b), nil
	}
	n := len(b)
	p.current += int64(n)
	p.signalProgress()
	p.report(false)
	return n, nil
}

func (p *progressReporter) finish(err error) {
	if p.writer == nil {
		p.doneOnce.Do(func() { close(p.doneCh) })
		return
	}
	p.doneOnce.Do(func() {
		p.report(true)
		if err == nil {
			fmt.Fprintln(p.writer)
		} else {
			fmt.Fprintln(p.writer, " (failed)")
		}
		close(p.doneCh)
	})
}

func (p *progressReporter) report(force bool) {
	if p.writer == nil {
		return
	}
	if !force && time.Since(p.lastUpdate) < 200*time.Millisecond {
		return
	}
	p.lastUpdate = time.Now()

	var status string
	if p.total > 0 {
		percent := float64(p.current) / float64(p.total) * 100
		status = fmt.Sprintf("%.1f%% (%s/%s)", percent, humanBytes(p.current), humanBytes(p.total))
	} else {
		status = fmt.Sprintf("%s downloaded", humanBytes(p.current))
	}
	fmt.Fprintf(p.writer, "\r%s", i18n.Tf("geoip.ip2region.downloading", map[string]interface{}{"Source": p.source, "Status": status}))
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 5 {
		div *= unit
		exp++
	}
	value := float64(n) / float64(div)
	return fmt.Sprintf("%.1f%ciB", value, "KMGTPE"[exp])
}

func detectIPVersion(dbPath string) (*xdb.Version, error) {
	handle, err := os.Open(dbPath)
	if err != nil {
		return nil, errors.New(i18n.Tf("geoip.ip2region.openFailed", map[string]interface{}{"Error": err.Error()}))
	}
	defer handle.Close()

	if err := xdb.Verify(handle); err != nil {
		return nil, errors.New(i18n.Tf("geoip.ip2region.verifyFailed", map[string]interface{}{"Error": err.Error()}))
	}

	header, err := xdb.LoadHeader(handle)
	if err != nil {
		return nil, errors.New(i18n.Tf("geoip.ip2region.headerFailed", map[string]interface{}{"Error": err.Error()}))
	}

	version, err := xdb.VersionFromHeader(header)
	if err != nil {
		return nil, errors.New(i18n.Tf("geoip.ip2region.versionFailed", map[string]interface{}{"Error": err.Error()}))
	}
	return version, nil
}

// region 格式：
// - ip2region v2: 国家|区域|省份|城市|ISP（5 字段）
// - ip2region v4: 国家|省份|城市|ISP（4 字段）
// 未知项可能为 0
func parseIP2Region(region string) *GeoLocation {
	parts := strings.Split(region, "|")
	var country, province, city, isp string

	switch len(parts) {
	case 4:
		// ip2region v4 格式：国家|省份|城市|ISP
		country = normalizeIP2R(parts[0])
		province = normalizeIP2R(parts[1])
		city = normalizeIP2R(parts[2])
		isp = normalizeIP2R(parts[3])
	case 5:
		// ip2region v2 格式：国家|区域|省份|城市|ISP
		country = normalizeIP2R(parts[0])
		province = normalizeIP2R(parts[2])
		city = normalizeIP2R(parts[3])
		isp = normalizeIP2R(parts[4])
	default:
		return nil
	}

	if country == "" && province == "" && city == "" && isp == "" {
		return nil
	}
	return &GeoLocation{
		Country:  country,
		Province: province,
		City:     city,
		ISP:      isp,
	}
}

func normalizeIP2R(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return ""
	}
	return s
}
