# hitachi_ops_center_graphite
Инструмент для извлечения данных производительности СХД Hitachi через Ops Center Analyzer и записи их в Graphite
## Конфигурация
Формат конфигурационного файла YAML. Пример:
```
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
   visibleName: "I07-VSPG800-111111"

 - serialNumber: "222222"
   type: "husvm"
   visibleName: "I07-HUSVM-222222"
```
