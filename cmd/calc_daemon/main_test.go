package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	pb "ya_lms_expression_calc_two/grpc/proto_exchange"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

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

func TestInitTaskStatus(t *testing.T) {

	var RecordID int64 = 123

	statusKey, actualStatus := taskStatus.Load(RecordID)
	if actualStatus {
		t.Errorf("Task status is already initialized for RecordID: %d", RecordID)
	}

	initTaskStatus(RecordID)

	statusKey, actualStatus = taskStatus.Load(RecordID)
	expectedStatus := queued

	if !actualStatus {
		t.Errorf("Task status is not initialized after calling initTaskStatus for RecordID: %d", RecordID)
	}

	if statusKey != expectedStatus {
		t.Errorf("Expected task status %s, but got %s", expectedStatus, statusKey)
	}
}

func TestSetTaskRunning(t *testing.T) {
	recordID := int64(123)

	setTaskRunning(recordID)

	statusKey, _ := taskStatus.Load(recordID)
	expectedStatus := running

	if statusKey != expectedStatus {
		t.Errorf("Expected task status to be '%s', got '%s'", expectedStatus, statusKey)
	}
}

func TestSetTaskCompleted(t *testing.T) {

	recordID := int64(123)

	setTaskCompleted(recordID)

	statusKey, _ := taskStatus.Load(recordID)
	expectedStatus := completed

	if statusKey != expectedStatus {
		t.Errorf("Expected task status to be '%s', got '%s'", expectedStatus, statusKey)
	}
}

func TestIsTaskQueued(t *testing.T) {

	recordID := int64(123)

	taskStatus.Store(recordID, queued)

	if !isTaskQueued(recordID) {
		t.Errorf("Expected task with RecordID %d to be queued", recordID)
	}

	taskStatus.Store(recordID, running)

	if isTaskQueued(recordID) {
		t.Errorf("Expected task with RecordID %d not to be queued", recordID)
	}

	// Не существующая задача
	if isTaskQueued(456) {
		t.Errorf("Expected non-existent task to not be queued")
	}
}

func TestConnectToGRPCServer(t *testing.T) {

	config := Config{
		ManagerGRPCServer:                  "localhost:50051",
		ManagerGRPCServerReconnectInterval: 5,
	}

	// Ожидание
	timer := time.NewTimer(3 * time.Second)

	// Канал для сигнала от таймера
	done := make(chan bool)

	go func() {
		client := connectToGRPCServer(config)
		if client != nil {
			done <- true
		}
	}()

	select {
	case <-done:
		// Клиент успешно создан
	case <-timer.C:
		// Время истекло, тест не пройден
		t.Error("Expected connection to take at least 3 seconds, but it took too short")
	}
}

//---------------

// Создаем заглушку для grpcClient
type mockGRPCClient struct{}

// Echo - имитация метода Echo клиента
func (m *mockGRPCClient) Echo(ctx context.Context, req *pb.EchoRequest, opts ...grpc.CallOption) (*pb.EchoResponse, error) {
	// Логику обработки вызова метода Echo в тестовом окружении
	return nil, nil
}

// Result - имитация метода Result клиента
func (m *mockGRPCClient) Result(ctx context.Context, req *pb.ResultRequest, opts ...grpc.CallOption) (*pb.ResultResponse, error) {
	// Логика обработки вызова метода Result в тестовом окружении
	return nil, nil
}

func TestSendEchoRequest_Success(t *testing.T) {

	config := &Config{
		ServerAddress:      "localhost",
		ServerPort:         "8080",
		MaxLenghtCalcQueue: 10,
		EchoRepeatInterval: 3,
		ManagerGRPCServer:  "localhost:50051",
	}

	grpcClient = &mockGRPCClient{}

	err := sendEchoRequest(config)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// mockGRPCClientWithError - заглушка gRPC клиента с ошибкой
type mockGRPCClientWithError struct{}

// Echo - имитация метода Echo клиента с возвращением ошибки
func (m *mockGRPCClientWithError) Echo(ctx context.Context, req *pb.EchoRequest, opts ...grpc.CallOption) (*pb.EchoResponse, error) {
	// Возвращаем ошибку для проверки обработки ошибок
	return nil, errors.New("test error")
}

// Result - имитация метода Result клиента
func (m *mockGRPCClientWithError) Result(ctx context.Context, req *pb.ResultRequest, opts ...grpc.CallOption) (*pb.ResultResponse, error) {
	/// Логика обработки вызова метода Result в тестовом окружении
	return nil, nil
}

func TestSendEchoRequest_Error(t *testing.T) {

	config := &Config{
		ServerAddress:      "localhost",
		ServerPort:         "8080",
		MaxLenghtCalcQueue: 10,
		EchoRepeatInterval: 3,
		ManagerGRPCServer:  "localhost:50051",
	}

	// Имитируем gRPC клиент, который возвращает ошибку
	grpcClient = &mockGRPCClientWithError{}

	// Вызываем тестируемую функцию
	err := sendEchoRequest(config)

	// Проверяем, что ошибка не равна nil
	if err == nil {
		t.Error("Expected an error, got nil")
	}

	// Проверяем, что возвращается ожидаемая ошибка
	expectedErr := errors.New("test error")
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

//---------------

func TestAddTask_AlreadyInQueue(t *testing.T) {
	config := Config{
		MaxLenghtCalcQueue: 10,
	}
	server := NewServer(config)

	// Создаем задачу, которая уже есть в очереди
	task := &pb.AddTaskRequest{
		RecordId:      123,
		TaskId:        456,
		Expression:    "2+3*4",
		RpnExpression: "234*+",
	}
	addTaskToCache(task.RecordId, &Task{}) // Помещаем задачу в кэш, чтобы эмулировать существующую задачу

	// Отправляем запрос на добавление задачи
	response, err := server.AddTask(context.Background(), task)

	// Проверяем, что задача уже в очереди и ответ "OK"
	assert.NoError(t, err)
	assert.Equal(t, &pb.AddTaskResponse{Status: "OK"}, response)
}

func TestAddTask_QueueNotFull(t *testing.T) {
	config := Config{
		MaxLenghtCalcQueue: 10,
	}
	server := NewServer(config)

	// Создаем задачу
	task := &pb.AddTaskRequest{
		RecordId:      123,
		TaskId:        456,
		Expression:    "2+3*4",
		RpnExpression: "234*+",
	}

	// Отправляем запрос на добавление задачи
	response, err := server.AddTask(context.Background(), task)

	// Проверяем, что задача успешно добавлена и ответ "OK"
	assert.NoError(t, err)
	assert.Equal(t, &pb.AddTaskResponse{Status: "OK"}, response)
}

func TestAddTask_QueueFull(t *testing.T) {
	config := Config{
		MaxLenghtCalcQueue: 1, // Очередь сделаем слишком маленькой
	}
	server := NewServer(config)

	// Создаем первую задачу, чтобы заполнить очередь
	task1 := &pb.AddTaskRequest{
		RecordId:      123,
		TaskId:        456,
		Expression:    "2+3*4",
		RpnExpression: "234*+",
	}
	_, _ = server.AddTask(context.Background(), task1)

	// Создаем вторую задачу
	task2 := &pb.AddTaskRequest{
		RecordId:      789,
		TaskId:        101,
		Expression:    "5*6",
		RpnExpression: "56*",
	}

	// Отправляем запрос на добавление второй задачи
	response, err := server.AddTask(context.Background(), task2)

	// Проверяем, что очередь заполнена и ответ "FULL"
	assert.NoError(t, err)
	assert.Equal(t, &pb.AddTaskResponse{Status: "FULL"}, response)
}

var mutex sync.Mutex

func TestAddTaskToCache(t *testing.T) {

	calcQueue = sync.Map{}
	taskStatus = sync.Map{}
	qntProcessedTasks = 0

	recordID := int64(123)
	task := &Task{
		RecordID: recordID,
	}

	// Используем мьютексы для захвата и освобождения блокировки
	mutex.Lock()
	addTaskToCache(recordID, task)
	mutex.Unlock()

	// Проверяем результаты после освобождения блокировки
	mutex.Lock()
	defer mutex.Unlock()

	// Проверяем, что задача добавлена в кэш
	storedTask, found := calcQueue.Load(recordID)
	assert.True(t, found)
	assert.Equal(t, task, storedTask)

	// Проверяем, что задача имеет начальное состояние
	statusKey, ok := taskStatus.Load(recordID)
	assert.True(t, ok)
	assert.Equal(t, queued, statusKey)

	// Проверяем, что количество обработанных задач увеличилось на 1
	assert.Equal(t, 1, qntProcessedTasks)

}

func TestCalcQueueSize_EmptyQueue(t *testing.T) {
	calcQueue = sync.Map{}
	size := calcQueueSize()
	assert.Equal(t, 0, size)
}

func TestCalcQueueSize_NonEmptyQueue(t *testing.T) {

	// Создаем непустую очередь
	calcQueue = sync.Map{}
	calcQueue.Store(int64(1), &Task{})
	calcQueue.Store(int64(2), &Task{})
	calcQueue.Store(int64(3), &Task{})

	size := calcQueueSize()

	assert.Equal(t, 3, size)
}

func TestGetTaskFromCache_RecordIDNotFound(t *testing.T) {

	calcQueue = sync.Map{}
	task, found := getTaskFromCache(123)

	assert.Nil(t, task)
	assert.False(t, found)
}

func TestGetTaskFromCache_RecordIDFound(t *testing.T) {

	task := &Task{RecordID: 123, TaskID: 456}
	calcQueue = sync.Map{}
	calcQueue.Store(int64(123), task)

	foundTask, found := getTaskFromCache(123)

	assert.NotNil(t, foundTask)
	assert.True(t, found)
	assert.Equal(t, task, foundTask)
}

func TestGetTaskFromCache_InvalidTaskType(t *testing.T) {

	invalidTask := "invalid"
	calcQueue = sync.Map{}
	calcQueue.Store(int64(123), &invalidTask)

	task, found := getTaskFromCache(123)

	assert.Nil(t, task)
	assert.False(t, found)
}

func TestIsOperator_Positive(t *testing.T) {
	operators := []string{"+", "-", "*", "/", "^"}
	for _, op := range operators {
		if !isOperator(op) {
			t.Errorf("Expected %s to be recognized as an operator", op)
		}
	}
}

func TestIsOperator_Negative(t *testing.T) {
	nonOperators := []string{"", " ", "x", "(", ")", "123", "abc"}
	for _, op := range nonOperators {
		if isOperator(op) {
			t.Errorf("Expected %s not to be recognized as an operator", op)
		}
	}
}

func TestExecuteOperation_Addition(t *testing.T) {
	result, err := executeOperation(&Config{}, "2", "3", "+")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected result to be 5, got %f", result)
	}
}

func TestExecuteOperation_Subtraction(t *testing.T) {
	result, err := executeOperation(&Config{}, "5", "3", "-")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 2 {
		t.Errorf("Expected result to be 2, got %f", result)
	}
}

func TestExecuteOperation_Multiplication(t *testing.T) {
	result, err := executeOperation(&Config{}, "2", "3", "*")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 6 {
		t.Errorf("Expected result to be 6, got %f", result)
	}
}

func TestExecuteOperation_Division(t *testing.T) {
	result, err := executeOperation(&Config{}, "6", "3", "/")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 2 {
		t.Errorf("Expected result to be 2, got %f", result)
	}
}

func TestExecuteOperation_DivisionByZero(t *testing.T) {
	_, err := executeOperation(&Config{}, "6", "0", "/")
	if err == nil {
		t.Error("Expected division by zero error, but got nil")
	}
	if err.Error() != "division by zero" {
		t.Errorf("Expected error message 'division by zero', got '%s'", err.Error())
	}
}

func TestExecuteOperation_Power(t *testing.T) {
	result, err := executeOperation(&Config{}, "2", "3", "^")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 8 {
		t.Errorf("Expected result to be 8, got %f", result)
	}
}

func TestExecuteOperation_ErrorParsingOperands(t *testing.T) {
	_, err := executeOperation(&Config{}, "abc", "3", "+")
	if err == nil {
		t.Error("Expected error parsing operands, but got nil")
	}
	if err.Error() != "error parsing operands" {
		t.Errorf("Expected error message 'error parsing operands', got '%s'", err.Error())
	}
}

func TestExecuteOperation_UnknownOperation(t *testing.T) {
	_, err := executeOperation(&Config{}, "2", "3", "&")
	if err == nil {
		t.Error("Expected unknown operation error, but got nil")
	}
	if err.Error() != "unknown operation" {
		t.Errorf("Expected error message 'unknown operation', got '%s'", err.Error())
	}
}

func TestExecSubExpression(t *testing.T) {
	config := &Config{
		MaxGoroutinesInCalcProcess: 2,
	}

	subExpr := make(map[string]RPNTuple)
	subExpr["1"] = RPNTuple{TokenA: "2", TokenB: "3", Operation: "+"}
	subExpr["2"] = RPNTuple{TokenA: "4", TokenB: "2", Operation: "-"}

	execSubExpression(subExpr, config)

	expectedResults := map[string]float64{
		"1": 5, // 2 + 3
		"2": 2, // 4 - 2
	}

	for key, value := range subExpr {
		expectedResult, ok := expectedResults[key]
		if !ok {
			t.Errorf("Key %s not found in expected results", key)
		}

		if value.Result != expectedResult {
			t.Errorf("Expected result for key %s to be %f, got %f", key, expectedResult, value.Result)
		}
	}
}

func TestEvaluateRPN(t *testing.T) {

	config := &Config{}
	config.AdditionalDelayInCalcOperation = 1
	config.MaxGoroutinesInCalcProcess = 3

	t.Run("Test valid RPN expression", func(t *testing.T) {
		task := &Task{}

		tokens := []string{"2", "3", "+"}
		result, err := evaluateRPN(tokens, task, config)
		expected := 5.0

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if result != expected {
			t.Errorf("Expected result %f, got %f", expected, result)
		}
	})

	t.Run("Test invalid RPN expression", func(t *testing.T) {
		task := &Task{}

		tokens := []string{"2", "3", "%"}
		_, err := evaluateRPN(tokens, task, config)

		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})
}

func TestCalculateExpressionFromTask_Success(t *testing.T) {

	task := &Task{
		RecordID:      123,
		TaskID:        456,
		Expression:    "1 + 2",
		RPNexpression: "1 2 +",
		IsWrong:       false,
		IsFinished:    false,
		Comment:       "",
		Result:        0,
	}

	config := &Config{}
	config.AdditionalDelayInCalcOperation = 1
	config.MaxGoroutinesInCalcProcess = 3

	calculateExpressionFromTask(task, config)

	// Проверка ожидаемых результатов
	assert.False(t, task.IsWrong, "Task should not be marked as wrong")
	assert.True(t, task.IsFinished, "Task should be marked as finished")
	assert.Equal(t, 3.0, task.Result, "Task result should be 3")
}

func TestCalculateExpressionFromTask_Error(t *testing.T) {

	task := &Task{
		RecordID:      123,
		TaskID:        456,
		Expression:    "1 / 0", // деление на ноль приведет к ошибке
		RPNexpression: "1 0 /",
		IsWrong:       false,
		IsFinished:    false,
		Comment:       "",
		Result:        0,
	}

	config := &Config{}
	config.AdditionalDelayInCalcOperation = 1
	config.MaxGoroutinesInCalcProcess = 3

	calculateExpressionFromTask(task, config)

	assert.True(t, task.IsWrong, "Задача должна быть помечена как неверная")
	assert.True(t, task.IsFinished, "Задача должна быть помечена как завершенная")

	// Сейчас возвращается общее сообщение
	// assert.Contains(t, task.Comment, "division by zero", "Сообщение об ошибке должно содержать 'division by zero'")
	assert.Contains(t, task.Comment, "error in calculating the operation", "Сообщение об ошибке должно содержать 'error in calculating the operation'")
}

func TestRunTask_EmptyExpression(t *testing.T) {

	recordID := int64(123)
	task := &Task{
		RecordID:      recordID,
		TaskID:        456,
		Expression:    "",      // Пустое выражение
		RPNexpression: "1 2 +", // Произвольное RPN-выражение
		IsWrong:       false,
		IsFinished:    false,
		Comment:       "",
		Result:        0,
	}

	config := &Config{}
	config.AdditionalDelayInCalcOperation = 1
	config.MaxGoroutinesInCalcProcess = 3

	runTask(recordID, task, config)

	// Проверка ожидаемых результатов
	assert.True(t, task.IsWrong, "Задача должна быть помечена как неверная")
	assert.True(t, task.IsFinished, "Задача должна быть помечена как завершенная")
	assert.Contains(t, task.Comment, fmt.Sprintf("Error expression for RecordID: %d", recordID),
		"Комментарий должен содержать сообщение об ошибке пустого выражения")
}

func TestRunTask_ParseRPNError(t *testing.T) {

	recordID := int64(123)
	task := &Task{
		RecordID:      recordID,
		TaskID:        456,
		Expression:    "+ 1 + +", // Некорректное выражение
		RPNexpression: "",        // Пустое RPN-выражение
		IsWrong:       false,
		IsFinished:    false,
		Comment:       "",
		Result:        0,
	}
	config := &Config{}
	config.AdditionalDelayInCalcOperation = 1
	config.MaxGoroutinesInCalcProcess = 3

	runTask(recordID, task, config)

	assert.True(t, task.IsWrong, "Задача должна быть помечена как неверная")
	assert.True(t, task.IsFinished, "Задача должна быть помечена как завершенная")

	// Нет специальной ошибки(нет спец проверки на корректность rpn лр его вычисления)
	assert.Contains(t, task.Comment, "error parsing final result", "Сообщение об ошибке должно содержать 'error parsing final result'")

}

func TestRunTask_CalculateExpression(t *testing.T) {

	recordID := int64(123)
	task := &Task{
		RecordID:      recordID,
		TaskID:        456,
		Expression:    "1 2 +", // Корректное выражение
		RPNexpression: "1 2 +", // Предоставленное RPN-выражение
		IsWrong:       false,
		IsFinished:    false,
		Comment:       "",
		Result:        0,
	}
	config := &Config{}
	config.AdditionalDelayInCalcOperation = 1
	config.MaxGoroutinesInCalcProcess = 3

	runTask(recordID, task, config)

	assert.False(t, task.IsWrong, "Задача не должна быть помечена как неверная")
	assert.True(t, task.IsFinished, "Задача должна быть помечена как завершенная")
	assert.Equal(t, 3.0, task.Result, "Результат вычисления выражения неверен")
	assert.Empty(t, task.Comment, "Не должно быть комментария об ошибке")
}

//-------------

func TestSetConfigInfo(t *testing.T) {
	config := &Config{}

	jsonBody := `{"settings": "{\"EchoRepeatInterval\": 5, \"MaxLenghtCalcQueue\": 100, \"DelayBasicLoop\": 10, \"DelayAddingLoop\": 2, \"MaxGoroutinesInCalcProcess\": 20, \"AdditionalDelayInCalcOperation\": 1}"}`

	req, err := http.NewRequest("POST", "/config", bytes.NewBuffer([]byte(jsonBody)))
	if err != nil {
		t.Fatal(err)
	}

	// Создаем тестовый ResponseWriter для записи ответа
	rr := httptest.NewRecorder()

	setConfigInfo(rr, req, config)

	if rr.Code != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	if config.EchoRepeatInterval != 5 || config.MaxLenghtCalcQueue != 100 ||
		config.DelayBasicLoop != 10 || config.DelayAddingLoop != 2 ||
		config.MaxGoroutinesInCalcProcess != 20 || config.AdditionalDelayInCalcOperation != 1 {
		t.Errorf("Handler did not set config settings correctly")
	}
}

func TestGetConfigAndStatisticsInfo(t *testing.T) {
	config := &Config{}
	config.ServerAddress = ""
	config.ServerPort = ""
	config.ConfigEndpoint = ""
	config.ManagerGRPCServer = ""
	config.ManagerGRPCServerReconnectInterval = 0
	config.EchoRepeatInterval = 5
	config.MaxLenghtCalcQueue = 3
	config.DelayBasicLoop = 10
	config.DelayAddingLoop = 2
	config.MaxGoroutinesInCalcProcess = 20
	config.AdditionalDelayInCalcOperation = 1

	_, err := http.NewRequest("GET", "/config", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// Устанавливаем начальное время и заполняем кэш задач для теста
	startTime = time.Now()
	calcQueue.Store(int64(123), &Task{
		RecordID:      123,
		TaskID:        105,
		Expression:    "2 + 2",
		RPNexpression: "2 2 +",
		IsFinished:    true,
		IsWrong:       false,
		Result:        4,
	})

	// Вызываем тестируемую функцию
	getConfigAndStatisticsInfo(rr, config)

	// Проверяем статус код ответа
	if rr.Code != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	info := ConfigInfo{
		StartTime:         startTime.Format(time.RFC3339),
		Uptime:            0,
		QntCurrentTasks:   1,
		QntProcessedTasks: 0,
		CurrentTasks: []TaskInfo{
			{
				RecordID:      123,
				TaskID:        105,
				Expression:    "2 + 2",
				RPNExpression: "2 2 +",
				IsFinished:    true,
				IsWrong:       false,
				Result:        "4",
			},
		},
		CurrentSettings: *config,
	}
	expectedJSON, _ := json.Marshal(info)
	expectedJSONStr := string(expectedJSON)

	respBody := rr.Body.String()
	if respBody != expectedJSONStr {
		t.Errorf("Handler returned unexpected body: got %v want %v", respBody, expectedJSONStr)
	}
}
