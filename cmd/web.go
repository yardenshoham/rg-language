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
		addr         string
		modelsDir    string
		audioCacheMB int
	)

	cmd := &cobra.Command{
		Use:     "web",
		Short:   "Start the web server",
		Example: "rg-language web --addr :25256",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := cmd.Context().Value(loggerKey{}).(*slog.Logger)
			if !cmd.Flags().Changed("addr") {
				addr = listenAddr(addr)
			}
			if !cmd.Flags().Changed("audio-cache-mb") {
				if mb, err := envInt("RG_AUDIO_CACHE_MB"); err != nil {
					return err
				} else if mb > 0 {
					audioCacheMB = mb
				}
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			// Loading takes a few seconds. Doing it before the port opens is what
			// keeps a deployment from taking traffic it cannot serve yet.
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

			return web.NewServer(logger, p).ListenAndServe(ctx, addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":25256", "Listen address ($RG_ADDR, or $PORT)")
	cmd.Flags().StringVar(&modelsDir, "models", modelsDirDefault(), "Directory holding the two .onnx models ($RG_MODELS_DIR)")
	cmd.Flags().IntVar(&audioCacheMB, "audio-cache-mb", 0, "Megabytes of synthesized audio to keep in memory ($RG_AUDIO_CACHE_MB)")
	return cmd
}

// listenAddr resolves the address from the environment. Railway injects PORT and
// expects the app to bind it.
func listenAddr(fallback string) string {
	if addr := os.Getenv("RG_ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return fallback
}

// envInt reads an optional integer from the environment, reporting a value that
// is set but unusable rather than silently ignoring it.
func envInt(name string) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return n, nil
}

func modelsDirDefault() string {
	if dir := os.Getenv("RG_MODELS_DIR"); dir != "" {
		return dir
	}
	return "/models"
}
