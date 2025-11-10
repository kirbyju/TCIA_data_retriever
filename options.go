package main

import (
	"fmt"
	"github.com/DavidGamba/go-getoptions"
	"os"
	"time"
)

var (
	DefaultProxy = "socks5://127.0.0.1:1080"
	ImageUrl     = "https://services.cancerimagingarchive.net/nbia-api/services/v4/getImage"
	MetaUrl      = "https://services.cancerimagingarchive.net/nbia-api/services/v4/getSeriesMetaData"
	SeriesUrl    = "https://services.cancerimagingarchive.net/nbia-api/services/v4/getSeries"
	TokenUrl     = "https://services.cancerimagingarchive.net/nbia-api/oauth/token"
)

type Options struct {
	Input           string
	Output          string
	Username        string
	Password        string
	Proxy           string
	Concurrent      int
	MaxRetries      int
	RetryDelay      time.Duration
	RequestDelay    time.Duration
	Meta            bool
	Force           bool
	NoDecompress    bool
	NoMD5           bool
	Version         bool
	Debug           bool
	MaxConnsPerHost int
	MetadataWorkers int
	RefreshMetadata bool
	SkipExisting    bool
	Auth            string
	SplitMetadata   bool
}

func InitOptions() *Options {
	var options Options
	var retryDelayStr, requestDelayStr string
	opt := getoptions.New()

	opt.StringVar(&options.Input, "i", "", opt.Description("input file (support .tcia, .csv, .tsv, .xlsx, .s5cmd)"))
	opt.StringVar(&options.Output, "o", "output", opt.Description("output directory"))
	opt.StringVar(&options.Username, "u", "", opt.Description("NBIA username"))
	opt.StringVar(&options.Password, "p", "", opt.Description("NBIA password"))
	opt.StringVar(&options.Proxy, "s", "", opt.Description("proxy server (e.g. socks5://127.0.0.1:1080)"))
	opt.IntVar(&options.Concurrent, "c", 10, opt.Description("concurrent downloads"))
	opt.IntVar(&options.MaxRetries, "r", 3, opt.Description("max retries for failed downloads"))
	opt.StringVar(&retryDelayStr, "retry-delay", "10s", opt.Description("delay between retries"))
	opt.StringVar(&requestDelayStr, "request-delay", "0s", opt.Description("delay between requests"))
	opt.BoolVar(&options.Meta, "m", false, opt.Description("only get meta information"))
	opt.BoolVar(&options.Force, "f", false, opt.Description("force re-download and extraction"))
	opt.BoolVar(&options.NoDecompress, "nd", false, opt.Description("do not decompress downloaded files"))
	opt.BoolVar(&options.NoMD5, "n5", false, opt.Description("do not perform MD5 checksum validation"))
	opt.BoolVar(&options.Version, "v", false, opt.Description("show version"))
	opt.BoolVar(&options.Debug, "d", false, opt.Description("debug mode"))
	opt.IntVar(&options.MaxConnsPerHost, "max-conns", 100, opt.Description("max connections per host"))
	opt.IntVar(&options.MetadataWorkers, "meta-workers", 10, opt.Description("concurrent metadata fetchers"))
	opt.BoolVar(&options.RefreshMetadata, "refresh-meta", false, opt.Description("force refresh of cached metadata"))
	opt.BoolVar(&options.SkipExisting, "skip-existing", true, opt.Description("skip download if file already exists"))
	opt.StringVar(&options.Auth, "auth", "", opt.Description("path to Gen3 API key file"))
	opt.BoolVar(&options.SplitMetadata, "split-metadata", false, opt.Description("split metadata into individual JSON files"))

	// aliases
	opt.Alias("i", "input")
	opt.Alias("o", "output")
	opt.Alias("u", "username")
	opt.Alias("p", "password")
	opt.Alias("s", "proxy")
	opt.Alias("c", "concurrent")
	opt.Alias("r", "retries")
	opt.Alias("m", "meta")
	opt.Alias("f", "force")
	opt.Alias("nd", "no-decompress")
	opt.Alias("n5", "no-md5")
	opt.Alias("v", "version")
	opt.Alias("d", "debug")

	_, err := opt.Parse(os.Args[1:])

	// Init Logger
	setLogger(options.Debug, "")

	if err != nil {
		logger.Error(err)
		fmt.Fprint(os.Stderr, opt.Help())
		os.Exit(1)
	}

	// Parse duration options
	retryDelay, err := time.ParseDuration(retryDelayStr)
	if err != nil {
		logger.Fatalf("Invalid retry delay: %v", err)
	}
	options.RetryDelay = retryDelay

	requestDelay, err := time.ParseDuration(requestDelayStr)
	if err != nil {
		logger.Fatalf("Invalid request delay: %v", err)
	}
	options.RequestDelay = requestDelay

	if options.Version {
		return &options
	}

	if options.Input == "" {
		logger.Error("input file is required")
		fmt.Fprint(os.Stderr, opt.Help())
		os.Exit(1)
	}

	// Validate input file exists
	if _, err := os.Stat(options.Input); os.IsNotExist(err) {
		logger.Fatalf("input file not found: %s", options.Input)
	}

	// Make sure concurrent downloads is at least 1
	if options.Concurrent < 1 {
		options.Concurrent = 1
	}
	if options.MetadataWorkers < 1 {
		options.MetadataWorkers = 1
	}

	// MD5 validation is on by default, if disabled, we use the old getImage endpoint
	if options.NoMD5 {
		logger.Infof("MD5 validation disabled")
	} else {
		ImageUrl = "https://services.cancerimagingarchive.net/nbia-api/services/v4/getImageWithMD5Hash"
		logger.Infof("Using MD5 validation endpoint")
	}

	return &options
}
