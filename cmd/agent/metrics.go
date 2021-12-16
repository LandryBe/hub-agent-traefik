package main

import (
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
	"github.com/traefik/neo-agent/pkg/logger"
	"github.com/traefik/neo-agent/pkg/metrics"
	"github.com/traefik/neo-agent/pkg/platform"
)

func newMetrics(token, platformURL string, cfg platform.MetricsConfig, cfgWatcher *platform.ConfigWatcher) (*metrics.Manager, *metrics.Store, error) {
	rc := retryablehttp.NewClient()
	rc.RetryWaitMin = time.Second
	rc.RetryWaitMax = 10 * time.Second
	rc.RetryMax = 4
	rc.Logger = logger.NewRetryableHTTPWrapper(log.Logger.With().Str("component", "metrics_client").Logger())

	httpClient := rc.StandardClient()

	client, err := metrics.NewClient(httpClient, platformURL, token)
	if err != nil {
		return nil, nil, err
	}

	store := metrics.NewStore()
	scraper := metrics.NewScraper(httpClient)

	mgr := metrics.NewManager(client, store, scraper)
	mgr.SetConfig(cfg.Interval, cfg.Tables)

	cfgWatcher.AddListener(func(cfg platform.Config) {
		mgr.SetConfig(cfg.Metrics.Interval, cfg.Metrics.Tables)
	})

	return mgr, store, nil
}
