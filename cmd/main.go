package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"file-manager/internal/adapters/localstorage"
	"file-manager/internal/adapters/server"
	"file-manager/internal/config"
	"file-manager/internal/usecases"
)

// shutdownTimeout - парни, если что тут небольшая оптимизация, это макс время для корретного завершения работы.
// это для предотвращения бесконечной блакировки, если соединения не закрываются вовремя.
const shutdownTimeout = 5 * time.Second

func main() {
	cfg := config.LoadConfig("config.yaml")

	// Надо убедиться, что директория существует прежде чем запускать сервер.
	// Грубо говоря, чтобы нам было куда записывать.
	if err := os.MkdirAll(cfg.Storage.BasePath, cfg.File.DirPermissions); err != nil {
		logrus.Fatalf("Failed to create storage directory: %v", err)
	}

	fileStorage := localstorage.NewLocalStorageService(cfg.Storage.BasePath, cfg.File.DirPermissions)
	fileUsecase := usecases.NewFileManagementUseCase(fileStorage, cfg)

	handler := server.NewHandler(
		fileUsecase,
		cfg.Static.Path,
		cfg.Static.TemplateFile,
		cfg.File.ForbiddenExtensions,
		cfg.Server.MaxUploadSize,
		cfg.Messages,
	)

	// регистрация всех маршрутов, они все настроены через config.yaml.
	// можно задать любые настройки без необходимости изменения кода.
	http.HandleFunc(cfg.Routes.Browse, handler.Browse)
	http.HandleFunc(cfg.Routes.BrowseAlt, handler.Browse)
	http.HandleFunc(cfg.Routes.Upload, handler.Upload)
	http.HandleFunc(cfg.Routes.CreateFolder, handler.CreateFolder)
	http.HandleFunc(cfg.Routes.Delete, handler.Delete)
	http.HandleFunc(cfg.Routes.Rename, handler.Rename)
	http.HandleFunc(cfg.Routes.Download, handler.Download)
	http.HandleFunc(cfg.Routes.DownloadFolder, handler.DownloadFolder)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: nil, // http.DefaultServeMux
	}

	// graceful shutdown.
	go func() {
		logrus.Infof("Server running on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logrus.Errorf("Server shutdown error: %v", err)
	} else {
		logrus.Info("Server stopped gracefully")
	}
}
