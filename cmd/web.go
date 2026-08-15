package cmd

import (
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
		addr          string
		modelsDir     string
		audioCacheMB  int
		posthogKey    string
		posthogHost   string
		posthogUIHost string
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
				if value := os.Getenv("RG_AUDIO_CACHE_MB"); value != "" {
					mb, err := strconv.Atoi(value)
					if err != nil {
						return fmt.Errorf("invalid RG_AUDIO_CACHE_MB: %w", err)
					}
					if mb > 0 {
						audioCacheMB = mb
					}
				}
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			// Loading takes a few seconds, and it happens before the port opens so a
			// deployment never takes traffic it cannot serve yet.
			started := time.Now()
			logger.Info("loading models", "dir", modelsDir)
			p, err := pipeline.New(ctx, pipeline.Options{
				ModelsDir:    modelsDir,
				AudioCacheMB: audioCacheMB,
			})
			if err != nil {
				return fmt.Errorf("loading models: %w", err)
			}
			defer func() {
				if err := p.Close(); err != nil {
					logger.Error("closing models", "error", err)
				}
			}()
			logger.Info("models loaded", "took", time.Since(started).Round(time.Millisecond))

			if posthogKey != "" {
				logger.Info("analytics enabled", "host", posthogHost)
			}

			return web.NewServer(logger, p, web.Config{
				PostHogKey:    posthogKey,
				PostHogHost:   posthogHost,
				PostHogUIHost: posthogUIHost,
			}).ListenAndServe(ctx, addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", listenAddr(), "Listen address ($RG_ADDR, or $PORT)")
	cmd.Flags().StringVar(&modelsDir, "models", modelsDirDefault(), "Directory holding the two .onnx models ($RG_MODELS_DIR)")
	cmd.Flags().IntVar(&audioCacheMB, "audio-cache-mb", 0, "Megabytes of synthesized audio to keep in memory ($RG_AUDIO_CACHE_MB)")
	// Analytics are opt-in: with no key the site serves no tracking script at all.
	cmd.Flags().StringVar(&posthogKey, "posthog-key", os.Getenv("RG_POSTHOG_KEY"), "PostHog project API key; enables analytics ($RG_POSTHOG_KEY)")
	cmd.Flags().StringVar(&posthogHost, "posthog-host", os.Getenv("RG_POSTHOG_HOST"), "PostHog ingestion host ($RG_POSTHOG_HOST)")
	cmd.Flags().StringVar(&posthogUIHost, "posthog-ui-host", os.Getenv("RG_POSTHOG_UI_HOST"), "PostHog dashboard host, for links when the ingestion host is a proxy ($RG_POSTHOG_UI_HOST)")
	return cmd
}

// listenAddr resolves the address from the environment; Railway injects PORT.
func listenAddr() string {
	if addr := os.Getenv("RG_ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":25256"
}

func modelsDirDefault() string {
	if dir := os.Getenv("RG_MODELS_DIR"); dir != "" {
		return dir
	}
	return "/models"
}
