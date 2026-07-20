# Hitachi Ops Center Graphite

Сервис для сбора метрик производительности и ёмкости СХД Hitachi через Ops Center Analyzer API с последующей отправкой данных в Graphite.

Проект основан на аналогичном сервисе [Hitachi Graphite](https://github.com/katrinvarf/hitachi_graphite) для Hitachi Tuning Manager, но реализован отдельно для работы с API Hitachi Ops Center Analyzer.

## Возможности

- сбор метрик производительности и ёмкости СХД Hitachi
- получение данных через Hitachi Ops Center Analyzer API
- отправка метрик в Graphite
- поддержка нескольких систем хранения
- параллельный сбор данных с использованием пула воркеров
- настройка конфигурации через YAML
- настройка списка собираемых метрик через отдельный файл
- логирование работы приложения

## Стек

- Go
- Hitachi Ops Center Analyzer API
- Graphite

## Конфигурация

Перед запуском необходимо создать файл `config.yml`.

В качестве примера можно использовать следующую структуру конфигурации:

```yaml
graphite:
 host: "127.0.0.1"
 port: 2003

analyzer_api:
 host: "192.168.226.169"
 port: 22015
 proto: "http"
 user: "user"
 password: "password"

workers:
 count: 8

logging:
 - logger: "CONSOLE"
   file: "stdout"
   level: "info"
   encoding: "text"

storages:
 - serialNumber: "111111"
   type: "g800"
   visibleName: "storage-g800-01"

 - serialNumber: "222222"
   type: "husvm"
   visibleName: "storage-husvm-01"
```

Параметры конфигурации:

* `graphite` — адрес и порт сервера Graphite
* `analyzer_api` — параметры подключения к Hitachi Ops Center Analyzer API
* `workers.count` — количество параллельных воркеров
* `logging` — параметры логирования
* `storages` — список систем хранения, с которых необходимо собирать метрики

### Собираемые метрики

Список собираемых метрик и параметры их обработки задаются в файле `config/metrics.yml`.

## Структура проекта

```text
.
├── config/        # загрузка и обработка конфигурации приложения
├── getData/       # получение и обработка данных из Ops Center Analyzer API
├── sendData/      # формирование и отправка метрик в Graphite
├── main.go        # точка входа в приложение
└── Dockerfile     # сборка Docker-образа
```

## Архитектура

Приложение получает данные из Hitachi Ops Center Analyzer API, преобразует их в формат Graphite и отправляет в систему мониторинга. Для ускорения обработки опрос нескольких систем хранения выполняется параллельно с использованием пула воркеров.
