package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	core_kafka "github.com/1C-L0V3R/golang-td-app/internal/core/broker/kafka"
	core_config "github.com/1C-L0V3R/golang-td-app/internal/core/config"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_mongo "github.com/1C-L0V3R/golang-td-app/internal/core/repository/mongo"
	projection_mongo_repository "github.com/1C-L0V3R/golang-td-app/internal/features/projection/repository/mongo"
	projection_service "github.com/1C-L0V3R/golang-td-app/internal/features/projection/service"
	"go.uber.org/zap"
)

func main() {
	cfg := core_config.NewConfigMust()
	time.Local = cfg.TimeZone

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	logger, err := core_logger.NewLogger(core_logger.NewConfigMust())
	if err != nil {
		fmt.Println("failed to init application logger: ", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Debug("application time zone", zap.Any("zone", time.Local))

	logger.Debug("initializing mongo client")
	mongoClient, err := core_mongo.NewClient(
		ctx,
		core_mongo.NewConfigMust(),
	)
	if err != nil {
		logger.Fatal("failed to init mongo client", zap.Error(err))
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			mongoClient.OpTimeout(),
		)
		defer closeCancel()

		if err := mongoClient.Close(closeCtx); err != nil {
			logger.Error("failed to close mongo client", zap.Error(err))
		}
	}()

	logger.Debug("initializing feature", zap.String("feature", "projection"))
	projectionRepository := projection_mongo_repository.NewProjectionRepository(mongoClient)

	if err := projectionRepository.EnsureIndexes(ctx); err != nil {
		logger.Fatal("failed to ensure mongo indexes", zap.Error(err))
	}

	projectionService := projection_service.NewProjectionService(
		projection_service.NewConfigMust(),
		core_kafka.NewConfigMust(),
		projectionRepository,
		logger,
	)
	defer func() {
		if err := projectionService.Close(); err != nil {
			logger.Error("failed to close projection consumer", zap.Error(err))
		}
	}()

	if err := projectionService.Run(ctx); err != nil {
		logger.Error("projection consumer run error", zap.Error(err))
	}
}
