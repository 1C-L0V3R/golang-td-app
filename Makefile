include .env
export

export PROJECT_ROOT=$(shell pwd)

env-up:
	@docker compose up -d todoapp-postgres todoapp-kafka todoapp-mongo

env-down:
	@docker compose down todoapp-postgres todoapp-kafka todoapp-mongo

env-cleanup:
	@read -p "Очистить все volume файлы окружения? ОПАСНОСТЬ УТЕРИ ДАННЫХ! [y/N]: " ans; \
	if [ "$$ans" = "y" ]; then \
		docker compose down todoapp-postgres todoapp-kafka todoapp-mongo port-forwarder mongo-port-forwarder && \
		rm -rf ${PROJECT_ROOT}/out/pgdata ${PROJECT_ROOT}/out/kafka ${PROJECT_ROOT}/out/mongodata && \
		echo "Файлы окружения очищены."; \
	else \
		echo "Очистка окружения отменена."; \
	fi

env-port-forward:
	@docker compose up -d port-forwarder mongo-port-forwarder

env-port-close:
	@docker compose down port-forwarder mongo-port-forwarder

migrate-create: 
	@if [ -z "$(seq)" ]; then \
		echo "Отсутствует необходимый параметр `seq`. Пример: make migrate-create seq=init."; \
		exit 1; \
	fi; \
	docker compose run --rm todoapp-postgres-migrate \
		create \
		-ext sql \
		-dir /migrations \
		-seq "$(seq)"

migrate-up:
	@make migrate-action action=up
	
migrate-down:
	@make migrate-action action=down

migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "Отсутствует необходимый параметр `action`. Пример: make migrate-action action=up."; \
		exit 1; \
	fi; \
	docker compose run --rm todoapp-postgres-migrate \
		-path /migrations \
		-database postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@todoapp-postgres:5432/${POSTGRES_DB}?sslmode=disable \
		"$(action)"

logs-cleanup:
	@read -p "Очистить все log файлы окружения? ОПАСНОСТЬ УТЕРИ ЛОГОВ! [y/N]: " ans; \
	if [ "$$ans" = "y" ]; then \
		rm -rf ${PROJECT_ROOT}/out/logs && \
		echo "Файлы логов очищены."; \
	else \
		echo "Очистка логов отменена."; \
	fi



todoapp-run:
	@export LOGGER_FOLDER=${PROJECT_ROOT}/out/logs && \
	export POSTGRES_HOST=localhost && \
	export KAFKA_BROKERS=localhost:29092 && \
	go mod tidy && \
	go run ${PROJECT_ROOT}/cmd/todoapp/main.go

todoapp-deploy:
	@docker compose up -d --build todoapp

todoapp-undeploy:
	@docker compose down todoapp

projector-run:
	@export LOGGER_FOLDER=${PROJECT_ROOT}/out/logs && \
	export KAFKA_BROKERS=localhost:29092 && \
	export MONGO_HOST=localhost && \
	go mod tidy && \
	go run ${PROJECT_ROOT}/cmd/projector/main.go

projector-deploy:
	@docker compose up -d --build projector

projector-undeploy:
	@docker compose down projector

swagger-gen:
	@docker compose run --rm swagger \
	init \
	-g cmd/todoapp/main.go \
	-o docs \
	--parseInternal \
	--parseDependency

ps: 
	@docker compose ps