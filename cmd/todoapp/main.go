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
	core_outbox_repository "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/outbox"
	core_pgx_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool/pgx"
	core_http_middleware "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/middleware"
	core_http_server "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/server"
	outbox_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/outbox/repository/postgres"
	outbox_service "github.com/1C-L0V3R/golang-td-app/internal/features/outbox/service"
	statistics_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/statistics/repository/postgres"
	statistics_service "github.com/1C-L0V3R/golang-td-app/internal/features/statistics/service"
	statistics_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/statistics/transport/http"
	tasks_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/repository/postgres"
	tasks_service "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/service"
	tasks_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/transport/http"
	users_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/users/repository/postgres"
	users_service "github.com/1C-L0V3R/golang-td-app/internal/features/users/service"
	users_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/users/transport/http"
	web_fs_repository "github.com/1C-L0V3R/golang-td-app/internal/features/web/repository/file_system"
	web_service "github.com/1C-L0V3R/golang-td-app/internal/features/web/service"
	web_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/web/transport/http"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	_ "github.com/1C-L0V3R/golang-td-app/docs"
)

// @title        Golang Todo API
// @version      1.0
// @description  Todo Application REST-API schema
// @host         127.0.0.1:5050
// @BasePath     /api/v1
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

	logger.Debug("initializing postgres connection pool")
	pool, err := core_pgx_pool.NewPool(
		ctx,
		core_pgx_pool.NewConfigMust(),
	)
	if err != nil {
		logger.Fatal("failed to init postgres connection pool", zap.Error(err))
	}
	defer pool.Close()

	logger.Debug("initializing kafka producer")
	kafkaConfig := core_kafka.NewConfigMust()
	kafkaProducer := core_kafka.NewProducer(kafkaConfig)
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			logger.Error("failed to close kafka producer", zap.Error(err))
		}
	}()

	outboxRepository := core_outbox_repository.NewOutboxRepository()

	logger.Debug("initializing feature", zap.String("feature", "users"))
	usersRepository := users_postgres_repository.NewUsersRepository(pool, outboxRepository)
	usersService := users_service.NewUsersService(usersRepository)
	usersTransportHTTP := users_transport_http.NewUsersHTTPHandler(usersService)

	logger.Debug("initializing feature", zap.String("feature", "tasks"))
	tasksRepository := tasks_postgres_repository.NewTasksRepository(pool, outboxRepository)
	tasksService := tasks_service.NewTasksService(tasksRepository)
	tasksTransportHTTP := tasks_transport_http.NewTasksHTTPHandler(tasksService)

	logger.Debug("initializing feature", zap.String("feature", "statistics"))
	statisticsRepository := statistics_postgres_repository.NewStatisticsRepository(pool)
	statisticsService := statistics_service.NewStatisticsService(statisticsRepository)
	statisticsTransportHTTP := statistics_transport_http.NewStatisticsHTTPHandler(statisticsService)

	logger.Debug("initializing feature", zap.String("feature", "web"))
	webRepository := web_fs_repository.NewWebRepository()
	webService := web_service.NewWebService(webRepository)
	webTransportHTTP := web_transport_http.NewWebHTTPHandler(webService)

	logger.Debug("initializing feature", zap.String("feature", "outbox"))
	outboxRelayRepository := outbox_postgres_repository.NewOutboxRepository(pool)
	outboxRelayService := outbox_service.NewRelayService(
		outbox_service.NewConfigMust(),
		pool,
		outboxRelayRepository,
		kafkaProducer,
		outbox_service.Topics{
			Users: kafkaConfig.UsersTopic(),
			Tasks: kafkaConfig.TasksTopic(),
		},
		logger,
	)

	logger.Debug("initializing HTTP server")
	httpServer := core_http_server.NewHTTPServer(
		core_http_server.NewConfigMust(),
		logger,
		core_http_middleware.CORS(),
		core_http_middleware.RequestID(),
		core_http_middleware.Logger(logger),
		core_http_middleware.Trace(),
		core_http_middleware.Panic(),
	)
	apiVersionRouterV1 := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	apiVersionRouterV1.RegisterRoutes(usersTransportHTTP.Routes()...)
	apiVersionRouterV1.RegisterRoutes(tasksTransportHTTP.Routes()...)
	apiVersionRouterV1.RegisterRoutes(statisticsTransportHTTP.Routes()...)

	// apiVersionRouterV2 := core_http_server.NewAPIVersionRouter(
	// 	core_http_server.ApiVersion2,
	// 	core_http_middleware.Dummy("api v2 middleware"),
	// )
	// apiVersionRouterV2.RegisterRoutes(usersTransportHTTP.Routes()...)

	httpServer.RegisterAPIRouters(
		apiVersionRouterV1,
		// apiVersionRouterV2,
	)
	httpServer.RegisterRoutes(webTransportHTTP.Routes()...)

	httpServer.RegisterSwagger()

	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		if err := httpServer.Run(groupCtx); err != nil {
			return fmt.Errorf("run HTTP server: %w", err)
		}

		return nil
	})

	group.Go(func() error {
		if err := outboxRelayService.Run(groupCtx); err != nil {
			return fmt.Errorf("run outbox relay: %w", err)
		}

		return nil
	})

	if err := group.Wait(); err != nil {
		logger.Error("application run error", zap.Error(err))
	}
}
