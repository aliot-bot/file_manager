# Тестирование

## Обзор

Проект содержит unit тесты для всех слоев архитектуры с использованием моков для изоляции зависимостей.

## Структура тестов

### 1. Usecases Layer (`internal/usecases/filemanagement_test.go`)

Тесты бизнес-логики с моками `FileStorage`:

- ✅ `TestNewFileManagementUseCase` - создание use case
- ✅ `TestFileManagementUseCase_sanitizePath` - валидация путей (path traversal, абсолютные пути, длина, символы)
- ✅ `TestFileManagementUseCase_List` - список файлов (успех, не найдено, нет доступа)
- ✅ `TestFileManagementUseCase_UploadFile` - загрузка файлов
- ✅ `TestFileManagementUseCase_Delete` - удаление файлов
- ✅ `TestFileManagementUseCase_Rename` - переименование
- ✅ `TestFileManagementUseCase_CreateFolder` - создание папок
- ✅ `TestFileManagementUseCase_shouldSkipFile` - пропуск скрытых файлов

**Моки:** `mockFileStorage`, `mockFileInfo`

**Покрытие:** 41.9% (основные функции покрыты)

---

### 2. LocalStorage Adapter (`internal/adapters/localstorage/localstorage_test.go`)

Интеграционные тесты с реальной файловой системой:

- ✅ `TestNewLocalStorageService` - создание сервиса
- ✅ `TestLocalStorageService_GetAbsolutePath` - построение абсолютных путей
- ✅ `TestLocalStorageService_ReadDirectory` - чтение директорий
- ✅ `TestLocalStorageService_WriteFile` - запись файлов (включая вложенные пути)
- ✅ `TestLocalStorageService_Remove` - удаление файлов и директорий
- ✅ `TestLocalStorageService_Move` - перемещение/переименование
- ✅ `TestLocalStorageService_CreateDirectory` - создание директорий
- ✅ `TestLocalStorageService_Integration` - интеграционный тест полного цикла

**Особенности:** Использует `t.TempDir()` для изоляции тестов

**Покрытие:** 83.9%

---

### 3. Server Handler (`internal/adapters/server/handler_test.go`)

Тесты HTTP handlers с моками `FileManagement`:

- ✅ `TestNewHandler` - создание handler
- ✅ `TestHandler_Browse` - просмотр директорий
- ✅ `TestHandler_Upload` - загрузка файлов (успех, запрещенные расширения, превышение размера)
- ✅ `TestHandler_CreateFolder` - создание папок
- ✅ `TestHandler_Delete` - удаление
- ✅ `TestHandler_Rename` - переименование
- ✅ `TestHandler_Download` - скачивание файлов
- ✅ `TestHandler_DownloadFolder` - скачивание папок
- ✅ `TestHandler_isForbidden` - проверка запрещенных расширений
- ✅ `TestHandler_getErrorType` - маппинг ошибок на HTTP статусы

**Моки:** `mockFileManagement`

**Покрытие:** 86.0%

---

### 4. Config (`internal/config/config_test.go`)

Тесты загрузки и валидации конфигурации:

- ✅ `TestLoadConfigWithError` - загрузка конфига (успех, файл не найден, невалидный YAML, отсутствующие поля, невалидный порт, невалидный размер)
- ✅ `TestValidateConfig` - валидация конфига
- ✅ `TestValidateRequiredString` - валидация строковых полей
- ✅ `TestValidatePort` - валидация портов
- ✅ `TestValidatePositiveInt` - валидация положительных чисел
- ✅ `TestValidatePositiveInt64` - валидация положительных int64
- ✅ `TestValidationError` - форматирование ошибок валидации

**Покрытие:** 88.9%

---

## Запуск тестов

### Все тесты
```bash
go test ./...
```

### С подробным выводом
```bash
go test ./... -v
```

### С покрытием кода
```bash
go test ./... -cover
```

### Детальный отчет о покрытии
```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out  # Откроет HTML отчет в браузере
```

### Тесты конкретного пакета
```bash
go test ./internal/usecases/...
go test ./internal/adapters/server/...
go test ./internal/adapters/localstorage/...
go test ./internal/config/...
```

## Покрытие кода

| Пакет | Покрытие |
|-------|----------|
| `internal/adapters/localstorage` | 83.9% |
| `internal/adapters/server` | 86.0% |
| `internal/config` | 88.9% |
| `internal/usecases` | 41.9% |
| **Общее** | **63.4%** |

## Моки

### mockFileStorage
Мок для интерфейса `domain.FileStorage`. Позволяет настраивать поведение каждого метода через функции-коллбэки.

**Использование:**
```go
mockStorage := &mockFileStorage{
    basePath: "/test",
    readDirectoryFunc: func(relPath string) ([]os.FileInfo, error) {
        return []os.FileInfo{...}, nil
    },
    writeFileFunc: func(relPath string, file io.Reader) error {
        return nil
    },
}
```

### mockFileManagement
Мок для интерфейса `domain.FileManagement`. Используется в тестах HTTP handlers.

**Использование:**
```go
mockUC := &mockFileManagement{
    listFunc: func(path string) ([]domain.FileData, error) {
        return []domain.FileData{...}, nil
    },
    uploadFileFunc: func(path string, file io.Reader) error {
        return nil
    },
}
```

### mockFileInfo
Мок для `os.FileInfo`. Используется для создания тестовых файловых объектов.

## Best Practices

1. **Изоляция:** Каждый тест изолирован и не зависит от других
2. **Моки:** Используются моки для всех внешних зависимостей
3. **Временные директории:** Используется `t.TempDir()` для файловых операций
4. **Table-driven tests:** Используются для тестирования множества сценариев
5. **Проверка ошибок:** Все тесты проверяют как успешные, так и ошибочные сценарии

## Добавление новых тестов

При добавлении новых функций:

1. Создайте тесты в соответствующем `*_test.go` файле
2. Используйте моки для изоляции зависимостей
3. Покройте как успешные, так и ошибочные сценарии
4. Используйте table-driven tests для множественных сценариев
5. Запустите `go test ./... -cover` для проверки покрытия

## Пример добавления теста

```go
func TestNewFeature(t *testing.T) {
    t.Run("success", func(t *testing.T) {
        // Arrange
        mockStorage := &mockFileStorage{...}
        uc := NewFileManagementUseCase(mockStorage, cfg)
        
        // Act
        result, err := uc.NewFeature("input")
        
        // Assert
        require.NoError(t, err)
        assert.Equal(t, expected, result)
    })
    
    t.Run("error case", func(t *testing.T) {
        // Test error scenarios
    })
}
```

