package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
)

//----------

func TestCreateDBConnection_Success(t *testing.T) {

	config := Config{}
	config.DBConnectionConfig.Host = "localhost"
	config.DBConnectionConfig.Port = 5432
	config.DBConnectionConfig.User = "admin"
	config.DBConnectionConfig.Password = "root"
	config.DBConnectionConfig.DBName = "postgres"

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer mockDB.Close()

	mock.ExpectPing()

	// Вызываем тестируемую функцию
	db, err := createDBConnection(config)

	// Проверяем, что ошибки нет
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Проверяем, что полученное соединение с базой данных не пустое
	if db == nil {
		t.Error("Expected database connection, got nil")
	}

	// Проверяем, что все ожидаемые вызовы были выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

//----------

func TestGetRequestIDFromHeader(t *testing.T) {

	// X-Request-ID присутствует и не пустой
	request := &http.Request{
		Header: http.Header{},
	}
	request.Header.Set("X-Request-ID", "1234567890")

	config := Config{MaxLenghtRequestId: 0}
	expectedID := "1234567890"
	expectedErr := error(nil)

	id, err := getRequestIDFromHeader(request, config)
	if id != expectedID || err != expectedErr {
		t.Errorf("Expected ID: %s, got: %s; Expected error: %v, got: %v", expectedID, id, expectedErr, err)
	}

	// X-Request-ID отсутствует
	request = &http.Request{
		Header: http.Header{},
	}
	expectedID = ""
	expectedErr = fmt.Errorf("X-Request-ID is missing or empty")

	id, err = getRequestIDFromHeader(request, config)
	if id != expectedID || err.Error() != expectedErr.Error() {
		t.Errorf("Expected ID: %s, got: %s; Expected error: %v, got: %v", expectedID, id, expectedErr, err)
	}

	// X-Request-ID превышает максимальную длину
	request = &http.Request{
		Header: http.Header{},
	}
	request.Header.Set("X-Request-ID", "12345678901234567890")
	config = Config{MaxLenghtRequestId: 10}
	expectedID = ""
	expectedErr = fmt.Errorf("X-Request-ID exceeds the maximum length of 10 characters")

	id, err = getRequestIDFromHeader(request, config)
	if id != expectedID || err.Error() != expectedErr.Error() {
		t.Errorf("Expected ID: %s, got: %s; Expected error: %v, got: %v", expectedID, id, expectedErr, err)
	}
}

//-------------

func TestReturnTaskIdToClient_Success(t *testing.T) {

	w := httptest.NewRecorder()
	expectedTaskID := "task123"

	err := returnTaskIdToClient(w, expectedTaskID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	expectedResponse := `{"task_id":"task123"}`
	if w.Body.String() != expectedResponse {
		t.Errorf("Expected response body %s, got %s", expectedResponse, w.Body.String())
	}
}

//-------------

func TestReturnTaskInfoToClient(t *testing.T) {

	taskInfo := &TaskInfoStruct{
		Expression: "1+2+3",
		Comment:    "Test comment",
		IsFinished: false,
	}

	w := httptest.NewRecorder()

	err := returnTaskInfoToClient(w, taskInfo)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var responseTaskInfo TaskInfoStruct
	if err := json.Unmarshal(w.Body.Bytes(), &responseTaskInfo); err != nil {
		t.Errorf("Error decoding response JSON: %v", err)
	}

	if !reflect.DeepEqual(taskInfo, &responseTaskInfo) {
		t.Errorf("Expected response task info to be %+v, got %+v", taskInfo, responseTaskInfo)
	}
}

//------------

func TestIsValidArithmeticExpression(t *testing.T) {

	config := Config{}
	config.PrepareValidationRegex = "^[0-9+\\-*/().^\\s]+$"

	// Корректное выражение
	expr := "2*(3+4)"
	if !isValidArithmeticExpression(expr, config) {
		t.Errorf("Expected expression '%s' to be valid", expr)
	}

	// Некорректное выражение
	invalidExpr := "2*(3+4N&)"
	if isValidArithmeticExpression(invalidExpr, config) {
		t.Errorf("Expected expression '%s' to be invalid", invalidExpr)
	}

	// Некорректное выражение (несбалансированные скобки)
	invalidExprWithBrackets := "2*(3+4"
	if isValidArithmeticExpression(invalidExprWithBrackets, config) {
		t.Errorf("Expected expression '%s' to be invalid", invalidExprWithBrackets)
	}
}

func TestIsValidArithmeticExpression_CustomValidationRegex(t *testing.T) {
	config := Config{}
	config.PrepareValidationRegex = "^[0-9+*]+$"

	// Корректное выражение
	expr := "2*3+4"
	if !isValidArithmeticExpression(expr, config) {
		t.Errorf("Expected expression '%s' to be valid", expr)
	}

	// Некорректное выражение(недопустимый символ и скобки)
	invalidExpr := "2*(3+4)^Я"
	if isValidArithmeticExpression(invalidExpr, config) {
		t.Errorf("Expected expression '%s' to be invalid", invalidExpr)
	}
}

//------------

func TestGetCurrExpression_ValidExpression(t *testing.T) {

	expression := "2+3*4"
	exprMessage := ExpressionMessage{Expression: expression}
	exprJSON, _ := json.Marshal(exprMessage)
	body := bytes.NewReader(exprJSON)

	req, err := http.NewRequest("POST", "/path", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	gotExpr, err := getCurrExpression(req, Config{})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if gotExpr != expression {
		t.Errorf("Expected expression '%s', got '%s'", expression, gotExpr)
	}
}

func TestGetCurrExpression_EmptyExpression(t *testing.T) {

	exprMessage := ExpressionMessage{Expression: ""}
	exprJSON, _ := json.Marshal(exprMessage)
	body := bytes.NewReader(exprJSON)

	req, err := http.NewRequest("POST", "/path", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	gotExpr, err := getCurrExpression(req, Config{})

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if gotExpr != "" {
		t.Errorf("Expected empty expression, got '%s'", gotExpr)
	}
	if !strings.Contains(err.Error(), "invalid arithmetic expression") {
		t.Errorf("Expected error message to contain 'invalid arithmetic expression', got '%s'", err.Error())
	}
}

func TestGetCurrExpression_ExpressionTooLong(t *testing.T) {

	expression := "1234567890123456789012345678901234567890"
	exprMessage := ExpressionMessage{Expression: expression}
	exprJSON, _ := json.Marshal(exprMessage)
	body := bytes.NewReader(exprJSON)

	req, err := http.NewRequest("POST", "/path", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	gotExpr, err := getCurrExpression(req, Config{MaxLenghtExpression: 30})

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if gotExpr != "" {
		t.Errorf("Expected empty expression, got '%s'", gotExpr)
	}
	if !strings.Contains(err.Error(), "expression exceeds the maximum length of 30 characters") {
		t.Errorf("Expected error message to contain 'expression exceeds the maximum length of 30 characters', got '%s'", err.Error())
	}
}

//------------

func TestGetTaskIDFromParams(t *testing.T) {
	taskID := "123"
	expectedTaskID := 123
	request := &http.Request{
		URL: &url.URL{
			RawQuery: "task_id=" + taskID,
		},
	}

	resultTaskID, err := getTaskIDFromParams(request)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resultTaskID != expectedTaskID {
		t.Errorf("Expected task ID '%d', got '%d'", expectedTaskID, resultTaskID)
	}
}

func TestGetTaskIDFromParams_MissingTaskID(t *testing.T) {
	request := &http.Request{
		URL: &url.URL{
			RawQuery: "",
		},
	}

	taskID, err := getTaskIDFromParams(request)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if taskID != 0 {
		t.Errorf("Expected task ID 0, got '%d'", taskID)
	}
}

func TestGetTaskIDFromParams_InvalidTaskID(t *testing.T) {

	invalidTaskID := "abc"
	request := &http.Request{
		URL: &url.URL{
			RawQuery: "task_id=" + invalidTaskID,
		},
	}

	taskID, err := getTaskIDFromParams(request)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if taskID != 0 {
		t.Errorf("Expected task ID 0, got '%d'", taskID)
	}
}

//------------

func TestInsertExpressionToDB(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	userID := 1
	requestID := "1234567890"
	currExpression := "2+3*4"
	expectedTaskID := "task123"

	mock.ExpectQuery("INSERT INTO reg_expr").
		WithArgs(userID, requestID, sqlmock.AnyArg(), currExpression).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedTaskID))

	taskID, err := insertExpressionToDB(mockDB, userID, requestID, currExpression)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if taskID != expectedTaskID {
		t.Errorf("Expected task ID '%s', got '%s'", expectedTaskID, taskID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

func TestInsertExpressionToDB_Error(t *testing.T) {

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	userID := 1
	requestID := "1234567890"
	currExpression := "2+3*4"

	mock.ExpectQuery("INSERT INTO reg_expr").
		WithArgs(userID, requestID, sqlmock.AnyArg(), currExpression).
		WillReturnError(sql.ErrNoRows)

	taskID, err := insertExpressionToDB(mockDB, userID, requestID, currExpression)

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if taskID != "" {
		t.Errorf("Expected empty task ID, got '%s'", taskID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

//------------

func TestSearchTaskIdByRequestId_LocalCacheHit(t *testing.T) {

	requestID := "1234567890"
	expectedTaskID := "task123"
	config := Config{UseLocalRequestIdCache: true}
	db, _, _ := sqlmock.New()
	requestIdCache.Store(requestID, expectedTaskID)

	taskID, err := searchTaskIdByRequestId(requestID, config, db)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if taskID != expectedTaskID {
		t.Errorf("Expected taskID: %s, got: %s", expectedTaskID, taskID)
	}
}

func TestSearchTaskIdByRequestId_LocalCacheMiss_DBHit(t *testing.T) {

	db, mock, _ := sqlmock.New()
	defer db.Close()

	requestID := "1234567890"
	expectedTaskID := "task123"
	config := Config{UseLocalRequestIdCache: true}

	mock.ExpectQuery("SELECT id FROM reg_expr WHERE request_id = ?").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedTaskID))

	taskID, err := searchTaskIdByRequestId(requestID, config, db)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if taskID != expectedTaskID {
		t.Errorf("Expected taskID: %s, got: %s", expectedTaskID, taskID)
	}
}

func TestSearchTaskIdByRequestId_LocalCacheDisabled_DBHit(t *testing.T) {

	db, mock, _ := sqlmock.New()
	defer db.Close()

	requestID := "1234567890"
	expectedTaskID := "task123"
	config := Config{UseLocalRequestIdCache: false}

	mock.ExpectQuery("SELECT id FROM reg_expr WHERE request_id = ?").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedTaskID))

	taskID, err := searchTaskIdByRequestId(requestID, config, db)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if taskID != expectedTaskID {
		t.Errorf("Expected taskID: %s, got: %s", expectedTaskID, taskID)
	}
}

func TestSearchTaskIdByRequestId_LocalCacheMiss_DBError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	requestID := "1234567890"
	config := Config{UseLocalRequestIdCache: true}
	expectedDBError := fmt.Errorf("database error")
	expectedError := fmt.Errorf("error getting result from database(searchRequestInDB): %v", expectedDBError)

	mock.ExpectQuery("SELECT id FROM reg_expr WHERE request_id = ?").
		WithArgs(requestID).
		WillReturnError(expectedDBError)

	_, err := searchTaskIdByRequestId(requestID, config, db)

	if err == nil || err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %v, got: %v", expectedError, err)
	}
}

//------------

func TestSearchRequestInDB(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	requestID := "1234567890"
	expectedTaskID := "task123"

	mock.ExpectQuery("SELECT id FROM reg_expr WHERE request_id = ?").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedTaskID))

	taskID, err := searchRequestInDB(db, requestID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if taskID != expectedTaskID {
		t.Errorf("Expected taskID: %s, got: %s", expectedTaskID, taskID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSearchRequestInDB_NoRows(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	requestID := "1234567890"

	mock.ExpectQuery("SELECT id FROM reg_expr WHERE request_id = ?").
		WithArgs(requestID).
		WillReturnError(sql.ErrNoRows)

	taskID, err := searchRequestInDB(db, requestID)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if taskID != "" {
		t.Errorf("Expected empty taskID, got: %s", taskID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSearchRequestInDB_Error(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	requestID := "1234567890"
	expectedError := sql.ErrConnDone

	mock.ExpectQuery("SELECT id FROM reg_expr WHERE request_id = ?").
		WithArgs(requestID).
		WillReturnError(expectedError)

	_, err = searchRequestInDB(db, requestID)

	if err != expectedError {
		t.Errorf("Expected error: %v, got: %v", expectedError, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

//------------

func TestGetUserByLogin(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock database: %v", err)
	}
	defer db.Close()

	testUser := &User{ID: 1, Login: "testuser", Password: "testpassword"}

	mock.ExpectQuery("SELECT id, login, password FROM users WHERE login = ?").
		WithArgs(testUser.Login).
		WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password"}).
			AddRow(testUser.ID, testUser.Login, testUser.Password))

	user, err := getUserByLogin(db, testUser.Login)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user == nil {
		t.Error("expected user, got nil")
	} else if *user != *testUser {
		t.Errorf("expected user: %+v, got: %+v", *testUser, *user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetUserByLogin_NoRows(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock database: %v", err)
	}
	defer db.Close()

	testUserLogin := "nonexistentuser"

	mock.ExpectQuery("SELECT id, login, password FROM users WHERE login = ?").
		WithArgs(testUserLogin).
		WillReturnError(sql.ErrNoRows)

	user, err := getUserByLogin(db, testUserLogin)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user, got: %+v", *user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetUserByLogin_Error(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock database: %v", err)
	}
	defer db.Close()

	testUserLogin := "testuser"

	mock.ExpectQuery("SELECT id, login, password FROM users WHERE login = ?").
		WithArgs(testUserLogin).
		WillReturnError(errors.New("database error"))

	user, err := getUserByLogin(db, testUserLogin)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if user != nil {
		t.Errorf("expected nil user, got: %+v", *user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

//------------

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expected     Config
		expectedErr  error
		setupFile    func(filename string) error // Функция для настройки теста: создание файла конфигурации
		cleanupFiles []string                    // Список файлов для удаления после завершения теста
	}{
		{
			name:     "ValidConfig",
			filename: "valid_config.json",
			expected: Config{ServerAddress: "value"},
			setupFile: func(filename string) error {

				// Создаем временный файл с валидной JSON конфигурацией
				file, err := os.Create(filename)
				if err != nil {
					return err
				}
				defer file.Close()

				encoder := json.NewEncoder(file)
				return encoder.Encode(Config{ServerAddress: "value"})
			},
			cleanupFiles: []string{"valid_config.json"},
		},
		{
			name:        "NonExistentFile",
			filename:    "non_existent_config.json",
			expectedErr: errors.New("open non_existent_config.json: no such file or directory"),
		},
		{
			name:     "InvalidJSON",
			filename: "invalid_json_config.json",
			setupFile: func(filename string) error {

				// Создаем временный файл с невалидной JSON конфигурацией
				file, err := os.Create(filename)
				if err != nil {
					return err
				}
				defer file.Close()

				_, err = file.WriteString("invalid_json")
				return err
			},
			expectedErr:  errors.New("invalid character 'i' looking for beginning of value"),
			cleanupFiles: []string{"invalid_json_config.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Выполняем настройку теста
			if tt.setupFile != nil {
				err := tt.setupFile(tt.filename)
				if err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			// Выполняем тестируемую функцию
			config, err := loadConfig(tt.filename)

			// Проверяем ожидаемые результаты
			if err != nil && tt.expectedErr == nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tt.expectedErr != nil {
				t.Fatalf("expected error: %v, got nil", tt.expectedErr)
			}
			if err != nil && err.Error() != tt.expectedErr.Error() {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}

			if config != tt.expected {
				t.Errorf("expected config: %+v, got: %+v", tt.expected, config)
			}

			// Выполняем очистку теста
			for _, filename := range tt.cleanupFiles {
				os.Remove(filename)
			}
		})
	}
}

//------------

func TestPushToStorage_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	// Создание тестовых данных
	config := Config{UseLocalRequestIdCache: true}
	userID := 1
	requestID := "1234567890"
	CurrExpression := "2+3*4"
	expectedTaskID := "task123"

	// Настройка мок-объекта для успешного выполнения вставки в базу данных
	mock.ExpectQuery("INSERT INTO reg_expr").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedTaskID))

	// Вызов функции
	taskID, err := pushToStorage(config, userID, requestID, CurrExpression, mockDB)

	// Проверка результата
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if taskID != expectedTaskID {
		t.Errorf("Expected task ID '%s', got '%s'", expectedTaskID, taskID)
	}

	// Проверка мок-объекта на соответствие ожиданиям
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}
