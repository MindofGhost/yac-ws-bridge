# Запуск подготовленного комплекта

Все команды выполняются из корня проекта на Linux VPS. Нужны Docker с Compose и Python 3. Для команд Yandex Cloud нужны настроенный `yc` CLI и `jq`.

Уже подготовлены:

- `compose.yaml`, `Dockerfile`: адаптер, непривилегированный пользователь, read-only файловая система, автоматический перезапуск.
- `deploy/spec.yaml` и конфиги: согласованные случайные пути; их список в `deploy/endpoints.json`.
- `deploy/adapter.config.yaml`, `deploy/helper.config.yaml`, `deploy/cloud.env`: один случайный общий секрет. Эти файлы исключены из Git. Сохраните их при переносе на VPS.
- `deploy/dist/bridge-function.zip`: минифицированная и обфусцированная функция без console-вызовов. В ZIP только `index.js` и `package.json`; оригинальный исходник сохранён в `bridge-cloud/index.js`.
- `deploy/dist/android/arm64-v8a/libhelper.so`: Android arm64 PIE executable, собранный с `-trimpath -s -w`. Это исполняемый процесс, а не JNI-библиотека. Go-бинарник не обфусцирован; обфускация в инструкции относится к JS-функции.

## 1. Подготовка и сборка

В этом рабочем каталоге сборка уже выполнена. После нового клонирования или изменения исходников:

```bash
python3 scripts/prepare-deploy.py
./scripts/build-release.sh
```

Повторная подготовка сохраняет существующий секрет и конфиги. Docker-сборка не включает конфиги с секретами. Скрипт сборки проверяет ответы исходной и обфусцированной функций (авторизация, HELLO, PING, SYNC, отключение), экспорт `handler` и отсутствие console-выводов.

## 2. Облако

Создайте сервисный аккаунт по разделу «Предварительные требования / Создание сервисного аккаунта» в `README_RU.md`, с ролями `serverless.functions.invoker` и `api-gateway.websocketBroadcaster`.

В `deploy/cloud.env` замените `YOUR_PUBLIC_HOST` на публичный HTTPS-домен адаптера, оставив случайный путь. Пример: `https://adapter.example.com/<сгенерированный-путь>`. Настройте на VPS HTTPS reverse proxy к `127.0.0.1:8080`, сохраняющий путь и заголовки `Authorization`, `X-IAM-Token`. Этот адрес должен быть доступен из Cloud Function. Для первого теста допустим HTTP с публичным IP и портом 8080, но HTTPS защищает передаваемые токены.

Создайте функцию и загрузите ZIP через консоль Yandex Cloud:

- Runtime: **Node.js 22** (`nodejs22`). Node.js 18 из оригинального README уже не поддерживается: https://yandex.cloud/ru/docs/functions/lang/nodejs/.
- Entrypoint: `index.handler`.
- Memory: 128 MB; timeout: 10 s; concurrency: 4.
- Service account: созданный сервисный аккаунт.
- Environment: значения `AUTH_TOKEN` и `HTTP_URL` из `deploy/cloud.env`.

Либо загрузите CLI (укажите ID своего аккаунта):

```bash
yc serverless function create --name bridge-fn
SA_ID='ID_СЕРВИСНОГО_АККАУНТА'
FUNCTION_ID=$(yc serverless function get bridge-fn --format json | jq -r .id)
set -a
. ./deploy/cloud.env
set +a
yc serverless function version create \
  --function-id "$FUNCTION_ID" \
  --runtime nodejs22 --entrypoint index.handler \
  --memory 128m --execution-timeout 10s --concurrency 4 \
  --source-path deploy/dist/bridge-function.zip \
  --service-account-id "$SA_ID" \
  --environment "AUTH_TOKEN=$AUTH_TOKEN,HTTP_URL=$HTTP_URL"
```

В `deploy/spec.yaml` замените `${FUNCTION_ID}` и `${SERVICE_ACCOUNT_ID}` на ID функции и сервисного аккаунта. Случайные пути и значения `context.route` уже настроены; оставьте их как есть.

```bash
yc serverless api-gateway create --name bridge-gw --spec deploy/spec.yaml
yc serverless api-gateway get bridge-gw --format json
```

Из результата возьмите `domain`. В обоих `deploy/*.config.yaml` замените только `YOUR_APIGW_ID.apigw.yandexcloud.net` на этот домен, сохранив `wss://` и путь. В MAUI-клиенте используйте полный URL из helper-конфига и тот же секрет.

## 3. Адаптер на VPS

В `deploy/adapter.config.yaml` укажите `target.address` своего TCP-сервиса. По умолчанию `127.0.0.1:1080`: на этом порту должен работать ваш SOCKS-прокси (Dante/XRay) или другой целевой сервис. Сам адаптер SOCKS-сервер не создаёт.

Compose использует сеть хоста Linux: `127.0.0.1` внутри адаптера относится к VPS. HTTP слушает порт 8080; порт должен быть свободен. Если поменяете `http.listenPort`, обновите reverse proxy / `HTTP_URL`.

```bash
./scripts/start-adapter.sh
docker compose logs --tail=50 adapter
```

Остановка / обновление:

```bash
docker compose down
./scripts/start-adapter.sh
```

Recovery endpoint отвечает `401` без Bearer-токена, `503` с корректным токеном до подключения к Gateway, `200` с JSON после подключения. Случайные пути не заменяют авторизацию. Обфускация не гарантирует незаметность трафика или отсутствие ограничений со стороны облака.

## 4. Хелпер в APK

Для собственного Android-приложения arm64:

1. Скопируйте `deploy/dist/android/arm64-v8a/libhelper.so` в `app/src/main/jniLibs/arm64-v8a/libhelper.so`.
2. Обеспечьте извлечение native libraries на диск при установке: `android:extractNativeLibs="true"` в `<application>` и `jniLibs.useLegacyPackaging = true` в Android Gradle packaging. Не передавайте файл в `System.loadLibrary`: это executable.
3. Приложение должно иметь `android.permission.INTERNET`. Сохраните настроенный helper-конфиг в приватный файл приложения; локальный адрес уже `127.0.0.1:5080`. Секрет из APK можно извлечь: предпочтительно получать конфиг после установки.
4. Запустите процесс из foreground service (права и тип сервиса подберите под targetSdk приложения), передав абсолютный путь конфигурации:

```kotlin
val helper = File(applicationInfo.nativeLibraryDir, "libhelper.so")
val config = File(filesDir, "helper.config.yaml") // предварительно запишите конфиг
val process = ProcessBuilder(helper.absolutePath, config.absolutePath)
    .redirectErrorStream(true)
    .start()
// Постоянно читайте process.inputStream в фоновом потоке, чтобы pipe не заполнился.
// При остановке сервиса: process.destroy().
```

Направьте TCP-клиент на `127.0.0.1:5080`. При использовании VPN/TUN исключите приложение с хелпером из туннелирования, чтобы не получить цикл. Это сборка только для arm64; на Android-устройстве она ещё не проверена. APK и интеграция сервиса требуют проекта вашего приложения.

Существующий `maui-client` реализует хелпер на C# и не требует встраивания Go-процесса; инструкции по его APK находятся в `maui-client/README_RU.md`.

Обновляйте функцию, адаптер и хелпер вместе.
