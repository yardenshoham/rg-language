package cmd

import (
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/yardenshoham/rg-language/internal/web"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

func newWebCmd() *cobra.Command {
	var (
		addr         string
		modelsDir    string
		audioCacheMB int
		config       web.Config
	)

	cmd := &cobra.Command{
		Use:     "web",
		Short:   "Start the web server",
		Example: "rg-language web --addr :25256",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			level := slog.LevelInfo
			if debug, _ := cmd.Flags().GetBool("debug"); debug {
				level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(cmd.OutOrStdout(), &slog.HandlerOptions{Level: level}))

			// Not a flag default: a set-but-unusable value should be reported.
			if !cmd.Flags().Changed("audio-cache-mb") {
				mb, err := strconv.Atoi(cmp.Or(os.Getenv("RG_AUDIO_CACHE_MB"), "0"))
				if err != nil {
					return fmt.Errorf("invalid RG_AUDIO_CACHE_MB: %w", err)
				}
				audioCacheMB = max(mb, 0)
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			// Before the port opens, so a deployment never takes traffic it cannot serve.
			started := time.Now()
			logger.Info("loading models", "dir", modelsDir)
			p, err := pipeline.New(ctx, modelsDir, audioCacheMB)
			if err != nil {
				return fmt.Errorf("loading models: %w", err)
			}
			defer p.Close() //nolint:errcheck // nothing useful to do about it here
			logger.Info("models loaded", "took", time.Since(started).Round(time.Millisecond))

			if config.PostHogKey != "" {
				logger.Info("analytics enabled", "host", config.PostHogHost)
			}

			return web.NewServer(logger, p, config).ListenAndServe(ctx, addr)
		},
	}

	// Railway injects PORT.
	cmd.Flags().StringVar(&addr, "addr", cmp.Or(os.Getenv("RG_ADDR"), ":"+cmp.Or(os.Getenv("PORT"), "25256")), "Listen address ($RG_ADDR, or $PORT)")
	cmd.Flags().StringVar(&modelsDir, "models", defaultModelsDir(), "Directory holding the two .onnx models ($RG_MODELS_DIR)")
	cmd.Flags().IntVar(&audioCacheMB, "audio-cache-mb", 0, "Megabytes of synthesized audio to keep in memory ($RG_AUDIO_CACHE_MB)")
	cmd.Flags().StringVar(&config.PostHogKey, "posthog-key", os.Getenv("RG_POSTHOG_KEY"), "PostHog project API key; enables analytics ($RG_POSTHOG_KEY)")
	cmd.Flags().StringVar(&config.PostHogHost, "posthog-host", os.Getenv("RG_POSTHOG_HOST"), "PostHog ingestion host ($RG_POSTHOG_HOST)")
	cmd.Flags().StringVar(&config.PostHogUIHost, "posthog-ui-host", os.Getenv("RG_POSTHOG_UI_HOST"), "PostHog dashboard host, for links when the ingestion host is a proxy ($RG_POSTHOG_UI_HOST)")
	return cmd
}

func defaultModelsDir() string { return cmp.Or(os.Getenv("RG_MODELS_DIR"), "/models") }
